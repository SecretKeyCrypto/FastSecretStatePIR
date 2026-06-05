// C++ Reed-Muller encoder (replaces GoRMEInterface.jl).
// Protocol: reads "input_path output_path q k m\n" lines from stdin,
// writes encoded CSV to output_path, signals done with "\n" to stdout.
//
// Algorithm:
//   Small d (d ≤ log₂q): Gaussian elimination + Newton-difference evaluation.
//   Large d (d >  log₂q): 2-D Newton forward-difference coefficient recovery
//                          (O(M), no Gaussian elimination) +
//                          binomial-convolution NTT evaluation (O(q² log q)).
//   Both paths: outer evaluation loop parallelised with std::async.
//
// CSV format:
//   2D: q lines, line x = P(x,0),...,P(x,q-1)
//   3D: q² lines, line x*q+y = P(x,y,0),...,P(x,y,q-1)

#include <algorithm>
#include <cassert>
#include <cmath>
#include <fstream>
#include <future>
#include <iostream>
#include <random>
#include <sstream>
#include <stdexcept>
#include <string>
#include <thread>
#include <vector>

// ─── portable parallel_for ─────────────────────────────────────────────────
// Splits [0, n) into chunks and runs f(i) in parallel via std::async.

static int hw_concurrency() {
#ifdef SINGLE_THREAD
    return 1;
#else
    int n = (int)std::thread::hardware_concurrency();
    return n > 0 ? n : 1;
#endif
}

template<typename F>
static void parallel_for(int n, F&& f) {
    if (n <= 0) return;
    int nt = std::min(hw_concurrency(), n);
    if (nt == 1) { for (int i = 0; i < n; ++i) f(i); return; }
    std::vector<std::future<void>> futs;
    futs.reserve(nt);
    int chunk = (n + nt - 1) / nt;
    for (int t = 0; t < nt; ++t) {
        int lo = t * chunk, hi = std::min(lo + chunk, n);
        if (lo >= n) break;
        futs.push_back(std::async(std::launch::async, [lo, hi, &f]() {
            for (int i = lo; i < hi; ++i) f(i);
        }));
    }
    for (auto& fut : futs) fut.get();
}

// ─── modular arithmetic (for PIR prime q) ─────────────────────────────────

static inline long long mod_q(long long v, int q) {
    return (v % q + q) % q;
}

static long long mpow(long long base, long long exp, long long q) {
    long long r = 1; base %= q;
    for (; exp > 0; exp >>= 1) {
        if (exp & 1) r = r * base % q;
        base = base * base % q;
    }
    return r;
}

static int modinv(int a, int q) { return (int)mpow(a, q - 2, q); }

// ─── binomial coefficient ──────────────────────────────────────────────────

static int binomial(int n, int k) {
    if (k < 0 || k > n) return 0;
    if (k == 0 || k == n) return 1;
    long long r = 1;
    for (int i = 0; i < k; ++i) r = r * (n - i) / (i + 1);
    return (int)r;
}

static int findSmallestD(int n, int m) {
    int d = std::max(1, (int)std::pow((double)n, 1.0 / m));
    while (binomial(m + d, d) < n) ++d;
    return d;
}

// ─── Gaussian elimination over F_q ────────────────────────────────────────

static std::vector<int> gaussElim(std::vector<std::vector<int>> A,
                                  std::vector<int> b, int q) {
    int n = (int)A.size();
    for (int i = 0; i < n; ++i) A[i].push_back(b[i]);
    for (int col = 0; col < n; ++col) {
        int pivot = -1;
        for (int row = col; row < n; ++row)
            if (A[row][col]) { pivot = row; break; }
        if (pivot < 0) throw std::runtime_error("singular Vandermonde");
        std::swap(A[col], A[pivot]);
        int inv = modinv(A[col][col], q);
        for (int j = col; j <= n; ++j)
            A[col][j] = (int)((long long)A[col][j] * inv % q);
        for (int row = 0; row < n; ++row) {
            if (row == col || A[row][col] == 0) continue;
            int f = A[row][col];
            for (int j = col; j <= n; ++j)
                A[row][j] = (int)mod_q((long long)A[row][j] - (long long)f * A[col][j], q);
        }
    }
    std::vector<int> x(n);
    for (int i = 0; i < n; ++i) x[i] = A[i][n];
    return x;
}

// ─── 1D Newton-difference evaluation ──────────────────────────────────────
// Σ poly[p]*x^p evaluated at x=0..q-1 via Newton forward differences.
// O(d² + q·d). Optimal for d < log₂(q).

static void evalPolyNewton(const std::vector<long long>& poly, int d, int q,
                           std::vector<int>& out) {
    std::vector<long long> diff(d + 1);
    for (int x = 0; x <= d; ++x) {
        long long v = 0;
        for (int p = d; p >= 0; --p) v = (v * x + poly[p]) % q;
        diff[x] = v;
    }
    for (int k = 1; k <= d; ++k)
        for (int j = d; j >= k; --j)
            diff[j] = (diff[j] - diff[j - 1] + q) % q;
    for (int x = 0; x < q; ++x) {
        out[x] = (int)diff[0];
        for (int k = 0; k < d; ++k) {
            diff[k] += diff[k + 1];
            if (diff[k] >= q) diff[k] -= q;
        }
    }
}

// ─── NTT over a prime field ────────────────────────────────────────────────
// Two NTT-friendly primes:
//   P1 = 998244353  = 119·2²³+1  (g=3, max NTT size 2²³ = 8 M)
//   P2 = 469762049  =   7·2²⁶+1  (g=3, max NTT size 2²⁶ = 64 M)
// P1·P2 ≈ 4.69×10¹⁷ > q³ for all q ≤ 65 521 → 2-prime CRT gives exact integers.

static const long long NP1 = 998244353LL;
static const long long NP2 = 469762049LL;
static const long long NG  = 3LL;   // primitive root mod both P1 and P2

static inline long long mulmod64(long long a, long long b, long long mod) {
    return (__int128)a * b % mod;
}

static void ntt(std::vector<long long>& a, bool inv, long long P) {
    int n = (int)a.size();
    for (int i = 1, j = 0; i < n; ++i) {
        int bit = n >> 1;
        for (; j & bit; bit >>= 1) j ^= bit;
        j ^= bit;
        if (i < j) std::swap(a[i], a[j]);
    }
    for (int len = 2; len <= n; len <<= 1) {
        long long w = inv ? mpow(NG, P - 1 - (P - 1) / len, P)
                          : mpow(NG, (P - 1) / len, P);
        for (int i = 0; i < n; i += len) {
            long long wn = 1;
            for (int j = 0; j < len / 2; ++j) {
                long long u = a[i + j];
                long long v = mulmod64(a[i + j + len / 2], wn, P);
                a[i + j]         = (u + v >= P) ? u + v - P : u + v;
                a[i + j + len/2] = (u >= v)     ? u - v     : u - v + P;
                wn = mulmod64(wn, w, P);
            }
        }
    }
    if (inv) {
        long long ni = mpow(n, P - 2, P);
        for (auto& x : a) x = mulmod64(x, ni, P);
    }
}

static int nttNextPow2(int x) { int n = 1; while (n < x) n <<= 1; return n; }

// ─── NTT state (precomputed once per parameter set) ───────────────────────

struct NTTState {
    int N, q;
    std::vector<long long> fact, inv_fact;
    std::vector<long long> gP1, gP2;  // NTT(inv_fact, P1/P2) of length N
    long long P1invP2;                 // P1⁻¹ mod P2 (CRT constant)
};

static NTTState buildNTTState(int q, int d) {
    NTTState st;
    st.q = q;
    st.N = nttNextPow2(d + q + 1);  // f has d+1 entries, g has q entries

    int need = st.N;
    st.fact.resize(need); st.inv_fact.resize(need);
    st.fact[0] = 1;
    for (int i = 1; i < need; ++i) st.fact[i] = st.fact[i-1] * i % q;
    st.inv_fact[need-1] = mpow(st.fact[need-1], q - 2, q);
    for (int i = need-2; i >= 0; --i)
        st.inv_fact[i] = st.inv_fact[i+1] * (i+1) % q;

    std::vector<long long> gvec(need, 0LL);
    for (int j = 0; j < q; ++j) gvec[j] = st.inv_fact[j];

    st.gP1 = gvec; ntt(st.gP1, false, NP1);
    st.gP2 = gvec; ntt(st.gP2, false, NP2);

    st.P1invP2 = mpow(NP1 % NP2, NP2 - 2, NP2);
    return st;
}

// ─── Evaluate Newton-basis polynomial via NTT binomial convolution ─────────
// Q(x) = Σ A[k] C(x,k) at x = 0..q-1.
// Identity: Q(x)/x! = (f ★ g)[x]  where f[k]=A[k]/k!, g[j]=1/j!.
// Uses 2-prime CRT NTT for exact modular recovery.

static void evalNewtonNTT(const std::vector<long long>& A, int deg,
                          const NTTState& st, std::vector<int>& out) {
    int N = st.N, q = st.q;

    std::vector<long long> fP1(N, 0), fP2(N, 0);
    for (int k = 0; k <= deg; ++k) {
        long long fk = A[k] * st.inv_fact[k] % q;
        fP1[k] = fk; fP2[k] = fk;
    }

    ntt(fP1, false, NP1); ntt(fP2, false, NP2);
    for (int i = 0; i < N; ++i) {
        fP1[i] = mulmod64(fP1[i], st.gP1[i], NP1);
        fP2[i] = mulmod64(fP2[i], st.gP2[i], NP2);
    }
    ntt(fP1, true, NP1); ntt(fP2, true, NP2);

    for (int x = 0; x < q; ++x) {
        long long h1 = fP1[x], h2 = fP2[x];
        // CRT: recover h mod P1*P2, then reduce mod q.
        long long diff = (h2 - h1 % NP2 + NP2) % NP2;
        long long t    = mulmod64(diff, st.P1invP2, NP2);
        long long hmodq = (h1 % q + (__int128)NP1 % q * (t % q)) % q;
        out[x] = (int)(st.fact[x] * hmodq % q);
    }
}

// ─── 2D Newton coefficient recovery (O(M)) ────────────────────────────────
// Converts triangle message values to Newton coefficients n[p][r] where
// P(x,y) = Σ_{p+r≤d} n[p][r] C(x,p) C(y,r).
// Triangle layout: y outer, x inner (column-major).

static std::vector<std::vector<long long>>
newton2DCoeffs(const std::vector<int>& msg, int d, int q) {
    // tri[r][p] = m(p, r) initially.
    std::vector<std::vector<long long>> tri(d + 1);
    int idx = 0;
    for (int r = 0; r <= d; ++r) {
        tri[r].resize(d - r + 1);
        for (int p = 0; p <= d - r; ++p) tri[r][p] = msg[idx++];
    }
    // Forward differences in x for each row r.
    for (int r = 0; r <= d; ++r) {
        int len = d - r + 1;
        for (int k = 1; k < len; ++k)
            for (int j = len-1; j >= k; --j)
                tri[r][j] = (tri[r][j] - tri[r][j-1] + q) % q;
    }
    // Forward differences in y for each column p.
    for (int p = 0; p <= d; ++p) {
        int len = d - p + 1;
        for (int k = 1; k < len; ++k)
            for (int j = len-1; j >= k; --j)
                tri[j][p] = (tri[j][p] - tri[j-1][p] + q) % q;
    }
    // tri[r][p] = n_{p,r}.
    return tri;
}

// ─── 2D systematic RM encoding ────────────────────────────────────────────

static std::vector<std::vector<int>> RME2D(const std::vector<int>& msg,
                                            int q, int d) {
    int N = binomial(d + 2, 2);
    assert((int)msg.size() == N);

    std::vector<std::vector<int>> matt(q, std::vector<int>(q));

    // NTT cost ∝ q² × log(q); Newton cost ∝ q² × d.
    // __int128 mulmod is ~10× slower than simple Newton adds, so NTT wins
    // only when d > ~20 × log₂(q).
    int log2q = 0; { int tmp = q; while (tmp >>= 1) ++log2q; }
    bool use_ntt = (d > 20 * log2q);

    if (!use_ntt) {
        // ── Newton path: O(M) coefficient recovery + O(qd) advance per row ──
        // nc[r][p] = 2D Newton coefficient for C(y,r)*C(x,p), computed via
        // forward differences in O(M) — no Gaussian elimination needed.
        auto nc = newton2DCoeffs(msg, d, q);

        // Precompute inv_fact[0..d] for binomial coefficient C(y,r).
        std::vector<long long> inv_fact(d+1);
        inv_fact[0] = 1;
        { long long f = 1; for (int i = 1; i <= d; ++i) f = f*i%q; inv_fact[d] = mpow(f, q-2, q); }
        for (int i = d-1; i >= 1; --i) inv_fact[i] = inv_fact[i+1] * (i+1) % q;

        parallel_for(q, [&](int y) {
            // C(y,r) = y*(y-1)*...*(y-r+1) / r!  mod q
            std::vector<long long> Cy(d+1);
            Cy[0] = 1;
            long long fall = 1;
            for (int r = 1; r <= d; ++r) {
                fall = fall * ((y - r + 1 + q) % q) % q;
                Cy[r] = fall * inv_fact[r] % q;
            }
            // A[p] = Σ_r nc[r][p] * C(y,r)  — these ARE the Newton diffs for Q_y(x)
            std::vector<long long> A(d+1, 0);
            for (int p = 0; p <= d; ++p)
                for (int r = 0; r <= d-p; ++r)
                    A[p] = (A[p] + nc[r][p] * Cy[r]) % q;
            // Newton advance: evaluate Q_y(x) for x=0..q-1
            for (int x = 0; x < q; ++x) {
                matt[x][y] = (int)A[0];
                for (int k = 0; k < d; ++k) { A[k] += A[k+1]; if (A[k] >= q) A[k] -= q; }
            }
        });

    } else {
        // ── NTT path (d > log q): Newton coefficients + NTT evaluation ──────
        // Phase 1: compute T[p][y] = A_p(y) = Σ_r n[p][r] C(y,r) for all p,y.
        auto ncoeffs = newton2DCoeffs(msg, d, q);
        NTTState st  = buildNTTState(q, d);

        std::vector<std::vector<int>> T(d+1, std::vector<int>(q));
        parallel_for(d+1, [&](int p) {
            int len = d - p + 1;
            std::vector<long long> Ap(len);
            for (int r = 0; r < len; ++r) Ap[r] = ncoeffs[r][p];
            evalNewtonNTT(Ap, len-1, st, T[p]);
        });

        // Phase 2: evaluate P(x,y) = Σ_p T[p][y] C(x,p) for all y.
        parallel_for(q, [&](int y) {
            std::vector<long long> Ay(d+1);
            for (int p = 0; p <= d; ++p) Ay[p] = T[p][y];
            std::vector<int> row(q);
            evalNewtonNTT(Ay, d, st, row);
            for (int x = 0; x < q; ++x) matt[x][y] = row[x];
        });
    }

    return matt;
}

// ─── 2D Lifted Reed-Solomon encoding ─────────────────────────────────────────
//
// Good-monomial condition for LRS(q, d) over F_q (prime q, 1 ≤ d ≤ q-1):
//   A monomial x^a y^b is "good" iff
//     (i)  0 ≤ a < d  and  0 ≤ b < d
//     (ii) a + b ≠ q-1
//
// Why condition (ii):  for a line with both coordinates active,
// x^a y^b restricted to the line evaluates as a constant × t^{a+b}.
// Over F_q the function t ↦ t^k has functional degree ((k-1) mod (q-1))+1.
// When a+b ≡ 0 (mod q-1) and a+b > 0, that degree equals q-1 ≥ d, violating
// the RS constraint.  The only value in {0,...,2(d-1)} ≡ 0 (mod q-1) with
// a+b > 0 is a+b = q-1 (since 2(d-1) < 2(q-1)).
//
// Message size: M = d² − max(0, 2d−q).   Rate → 1 as d → q-1.

static std::vector<std::pair<int,int>> goodMonomials2D(int q, int d) {
    std::vector<std::pair<int,int>> G;
    G.reserve(d * d);
    for (int a = 0; a < d; ++a)
        for (int b = 0; b < d; ++b)
            if (a + b != q - 1)
                G.emplace_back(a, b);
    return G;
}

// msg[i] = coefficient of the i-th good monomial (in the order returned by
// goodMonomials2D).  Encoding = evaluate f(x,y) = Σ c[a][b] x^a y^b at all
// (x,y) ∈ F_q².  Uses standard-basis Newton evaluation (no Gaussian elim).

static std::vector<std::vector<int>> LRSE2D(const std::vector<int>& msg, int q, int d) {
    auto G = goodMonomials2D(q, d);
    assert((int)msg.size() == (int)G.size());

    // Coefficient table: c[a][b] = 0 for bad monomials (already excluded).
    std::vector<std::vector<long long>> c(d, std::vector<long long>(d, 0));
    for (int i = 0; i < (int)G.size(); ++i)
        c[G[i].first][G[i].second] = msg[i];

    std::vector<std::vector<int>> cw(q, std::vector<int>(q));

    parallel_for(q, [&](int y) {
        // y^0, y^1, ..., y^{d-1}
        std::vector<long long> ypow(d);
        ypow[0] = 1;
        for (int b = 1; b < d; ++b) ypow[b] = ypow[b-1] * y % q;

        // A[a] = Σ_b c[a][b] · y^b  (coefficient of x^a in P_y(x))
        // Bad entries in c are 0, so no explicit skip needed.
        std::vector<long long> A(d, 0);
        for (int a = 0; a < d; ++a)
            for (int b = 0; b < d; ++b)
                A[a] = (A[a] + c[a][b] * ypow[b]) % q;

        // Evaluate P_y(x) = A[0] + A[1]x + ... + A[d-1]x^{d-1} at x=0..q-1.
        std::vector<int> row(q);
        evalPolyNewton(A, d - 1, q, row);
        for (int x = 0; x < q; ++x) cw[x][y] = row[x];
    });

    return cw;
}

// ─── 3D systematic RM encoding ────────────────────────────────────────────

static std::vector<std::vector<std::vector<int>>>
RME3D(const std::vector<int>& msg, int q, int d) {
    int N = binomial(d + 3, 3);
    assert((int)msg.size() == N);

    struct Mon3 { int p, r, s; };
    std::vector<Mon3> pts(N), mons(N);
    int idx = 0;
    for (int s = 0; s <= d; ++s)
        for (int r = 0; r <= d-s; ++r)
            for (int p = 0; p <= d-s-r; ++p)
                pts[idx] = mons[idx] = {p, r, s}, ++idx;

    std::vector<std::vector<int>> V(N, std::vector<int>(N));
    for (int i = 0; i < N; ++i)
        for (int j = 0; j < N; ++j)
            V[i][j] = (int)(mpow(pts[i].p, mons[j].p, q) *
                             mpow(pts[i].r, mons[j].r, q) % q *
                             mpow(pts[i].s, mons[j].s, q) % q);

    std::vector<int> b(msg.begin(), msg.end());
    auto c = gaussElim(V, b, q);

    std::vector<std::vector<std::vector<int>>>
        c3d(d+1, std::vector<std::vector<int>>(d+1, std::vector<int>(d+1, 0)));
    for (int j = 0; j < N; ++j)
        c3d[mons[j].p][mons[j].r][mons[j].s] = c[j];

    auto cw = std::vector<std::vector<std::vector<int>>>(
        q, std::vector<std::vector<int>>(q, std::vector<int>(q)));

    // Parallelize over z×y pairs.
    parallel_for(q * q, [&](int zy) {
        int z = zy / q, y = zy % q;
        std::vector<long long> Z(d+1), Y(d+1), A(d+1);
        std::vector<int> col(q);
        Z[0] = Y[0] = 1;
        for (int s = 1; s <= d; ++s) Z[s] = Z[s-1] * z % q;
        for (int r = 1; r <= d; ++r) Y[r] = Y[r-1] * y % q;
        for (int p = 0; p <= d; ++p) {
            long long a = 0;
            for (int r = 0; r <= d-p; ++r)
                for (int s = 0; s <= d-p-r; ++s)
                    a += (long long)c3d[p][r][s] * Y[r] % q * Z[s] % q;
            A[p] = a % q;
        }
        evalPolyNewton(A, d, q, col);
        for (int x = 0; x < q; ++x) cw[x][y][z] = col[x];
    });

    return cw;
}

// ─── CSV I/O ───────────────────────────────────────────────────────────────

static std::vector<int> readMessageCSV(const std::string& path) {
    std::ifstream f(path);
    if (!f) throw std::runtime_error("cannot open " + path);
    std::string line;
    if (!std::getline(f, line)) throw std::runtime_error("empty CSV: " + path);
    std::vector<int> msg; std::stringstream ss(line); std::string tok;
    while (std::getline(ss, tok, ',')) if (!tok.empty()) msg.push_back(std::stoi(tok));
    return msg;
}

static void writeCSV2D(const std::string& path,
                       const std::vector<std::vector<int>>& matt) {
    std::ofstream f(path);
    if (!f) throw std::runtime_error("cannot write " + path);
    int q = (int)matt.size();
    for (int x = 0; x < q; ++x) {
        for (int y = 0; y < q; ++y) { if (y) f << ','; f << matt[x][y]; }
        f << '\n';
    }
}

static void writeCSV3D(const std::string& path,
                       const std::vector<std::vector<std::vector<int>>>& cw) {
    std::ofstream f(path);
    if (!f) throw std::runtime_error("cannot write " + path);
    int q = (int)cw.size();
    for (int x = 0; x < q; ++x)
        for (int y = 0; y < q; ++y) {
            for (int z = 0; z < q; ++z) { if (z) f << ','; f << cw[x][y][z]; }
            f << '\n';
        }
}

// ─── main ─────────────────────────────────────────────────────────────────

int main() {
    std::mt19937 rng(std::random_device{}());
    std::string line;
    while (std::getline(std::cin, line)) {
        if (line.empty()) continue;
        std::istringstream ss(line);
        std::string inp, outp; int q, k, m;
        if (!(ss >> inp >> outp >> q >> k >> m)) continue;

        auto msg = readMessageCSV(inp);
        int n = (int)msg.size();

        if (m == 4) {
            // 2D Lifted RS: find smallest d with d²−max(0,2d−q) ≥ n
            int d = 1;
            while ((int)goodMonomials2D(q, d).size() < n) ++d;
            if (d * k + 1 >= q)
                throw std::runtime_error("parameters violate d*k+1 < q constraint");
            auto G = goodMonomials2D(q, d);
            std::uniform_int_distribution<int> dist(0, q-1);
            while ((int)msg.size() < (int)G.size()) msg.push_back(dist(rng));
            writeCSV2D(outp, LRSE2D(msg, q, d));
        } else {
            int d = findSmallestD(n, m);
            if (d * k + 1 >= q)
                throw std::runtime_error("parameters violate d*k+1 < q constraint");
            int N = binomial(m + d, d);
            std::uniform_int_distribution<int> dist(0, q-1);
            while ((int)msg.size() < N) msg.push_back(dist(rng));
            if (m == 2) { writeCSV2D(outp, RME2D(msg, q, d)); }
            else if (m == 3) { writeCSV3D(outp, RME3D(msg, q, d)); }
            else throw std::runtime_error("unsupported m=" + std::to_string(m));
        }

        std::cout << '\n'; std::cout.flush();
    }
}
