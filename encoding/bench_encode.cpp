// Standalone RM encoding benchmark.
// Compile:
//   c++ -O3 -std=c++17 -o bench_encode bench_encode.cpp        # multi-thread
//   c++ -O3 -std=c++17 -DSINGLE_THREAD -o bench_encode_st bench_encode.cpp

#include <algorithm>
#include <cassert>
#include <chrono>
#include <cstdio>
#include <future>
#include <random>
#include <string>
#include <thread>
#include <vector>

// ── Shared utilities (mirror of rme.cpp) ──────────────────────────────────

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

static long long mpow(long long b, long long e, long long q) {
    long long r = 1; b %= q;
    for (; e > 0; e >>= 1) { if (e & 1) r = r*b%q; b = b*b%q; }
    return r;
}
static int binomial(int n, int k) {
    if (k < 0 || k > n) return 0;
    if (!k || k == n) return 1;
    long long r = 1;
    for (int i = 0; i < k; ++i) r = r*(n-i)/(i+1);
    return (int)r;
}

static void evalPolyNewton(const std::vector<long long>& poly, int d, int q,
                            std::vector<int>& out) {
    std::vector<long long> diff(d+1);
    for(int x=0;x<=d;++x){long long v=0;for(int p=d;p>=0;--p)v=(v*x+poly[p])%q;diff[x]=v;}
    for(int k=1;k<=d;++k)for(int j=d;j>=k;--j)diff[j]=(diff[j]-diff[j-1]+q)%q;
    for(int x=0;x<q;++x){out[x]=(int)diff[0];for(int k=0;k<d;++k){diff[k]+=diff[k+1];if(diff[k]>=q)diff[k]-=q;}}
}

// LRS good monomials: (a,b) with 0 ≤ a,b < d and a+b ≠ q-1
static std::vector<std::pair<int,int>> goodMonomials2D(int q,int d){
    std::vector<std::pair<int,int>> G; G.reserve(d*d);
    for(int a=0;a<d;++a)for(int b=0;b<d;++b)if(a+b!=q-1)G.emplace_back(a,b);
    return G;
}
static long long LRSE2D(const std::vector<int>& msg,int q,int d){
    auto G=goodMonomials2D(q,d); assert((int)msg.size()==(int)G.size());
    std::vector<std::vector<long long>> c(d,std::vector<long long>(d,0));
    for(int i=0;i<(int)G.size();++i)c[G[i].first][G[i].second]=msg[i];
    std::vector<std::vector<int>> cw(q,std::vector<int>(q));
    parallel_for(q,[&](int y){
        std::vector<long long> ypow(d),A(d,0);
        ypow[0]=1;for(int b=1;b<d;++b)ypow[b]=ypow[b-1]*y%q;
        for(int a=0;a<d;++a)for(int b=0;b<d;++b)A[a]=(A[a]+c[a][b]*ypow[b])%q;
        std::vector<int> row(q); evalPolyNewton(A,d-1,q,row);
        for(int x=0;x<q;++x)cw[x][y]=row[x];
    });
    (void)cw;
    return (long long)q*q*2;
}

// NTT
static const long long NP1=998244353, NP2=469762049, NG=3;
static inline long long ml(long long a,long long b,long long P){return(__int128)a*b%P;}
static void ntt(std::vector<long long>& a, bool inv, long long P) {
    int n=(int)a.size();
    for(int i=1,j=0;i<n;++i){int bit=n>>1;for(;j&bit;bit>>=1)j^=bit;j^=bit;if(i<j)std::swap(a[i],a[j]);}
    for(int len=2;len<=n;len<<=1){
        long long w=inv?mpow(NG,P-1-(P-1)/len,P):mpow(NG,(P-1)/len,P);
        for(int i=0;i<n;i+=len){long long wn=1;for(int j=0;j<len/2;++j){
            long long u=a[i+j],v=ml(a[i+j+len/2],wn,P);
            a[i+j]=(u+v>=P)?u+v-P:u+v;a[i+j+len/2]=(u>=v)?u-v:u-v+P;wn=ml(wn,w,P);}}}
    if(inv){long long ni=mpow(n,P-2,P);for(auto&x:a)x=ml(x,ni,P);}
}
static int np2(int x){int n=1;while(n<x)n<<=1;return n;}

struct NTTState {
    int N,q; std::vector<long long> fact,inv_fact,gP1,gP2; long long P1invP2;
};
static NTTState buildNTT(int q, int d) {
    NTTState st; st.q=q; st.N=np2(d+q+1);
    int sz=st.N; st.fact.resize(sz); st.inv_fact.resize(sz);
    st.fact[0]=1; for(int i=1;i<sz;++i) st.fact[i]=st.fact[i-1]*i%q;
    st.inv_fact[sz-1]=mpow(st.fact[sz-1],q-2,q);
    for(int i=sz-2;i>=0;--i) st.inv_fact[i]=st.inv_fact[i+1]*(i+1)%q;
    std::vector<long long> g(sz,0);
    for(int j=0;j<q;++j) g[j]=st.inv_fact[j];
    st.gP1=g; ntt(st.gP1,false,NP1);
    st.gP2=g; ntt(st.gP2,false,NP2);
    st.P1invP2=mpow(NP1%NP2,NP2-2,NP2);
    return st;
}
static void evalNTT(const std::vector<long long>& A,int deg,const NTTState& st,std::vector<int>& out){
    int N=st.N,q=st.q;
    std::vector<long long> fP1(N,0),fP2(N,0);
    for(int k=0;k<=deg;++k){long long fk=A[k]*st.inv_fact[k]%q;fP1[k]=fP2[k]=fk;}
    ntt(fP1,false,NP1);ntt(fP2,false,NP2);
    for(int i=0;i<N;++i){fP1[i]=ml(fP1[i],st.gP1[i],NP1);fP2[i]=ml(fP2[i],st.gP2[i],NP2);}
    ntt(fP1,true,NP1);ntt(fP2,true,NP2);
    for(int x=0;x<q;++x){
        long long h1=fP1[x],h2=fP2[x];
        long long diff=(h2-h1%NP2+NP2)%NP2;
        long long t=ml(diff,st.P1invP2,NP2);
        long long hm=(h1%q+(__int128)NP1%q*(t%q))%q;
        out[x]=(int)(st.fact[x]*hm%q);
    }
}
static std::vector<std::vector<long long>> newton2D(const std::vector<int>& msg,int d,int q){
    std::vector<std::vector<long long>> tri(d+1);
    int idx=0;
    for(int r=0;r<=d;++r){tri[r].resize(d-r+1);for(int p=0;p<=d-r;++p)tri[r][p]=msg[idx++];}
    for(int r=0;r<=d;++r){int l=d-r+1;for(int k=1;k<l;++k)for(int j=l-1;j>=k;--j)tri[r][j]=(tri[r][j]-tri[r][j-1]+q)%q;}
    for(int p=0;p<=d;++p){int l=d-p+1;for(int k=1;k<l;++k)for(int j=l-1;j>=k;--j)tri[j][p]=(tri[j][p]-tri[j-1][p]+q)%q;}
    return tri;
}

// ─── 2D encode (returns codeword volume in bytes) ─────────────────────────

static long long RME2D(const std::vector<int>& msg, int q, int d) {
    int N=binomial(d+2,2); assert((int)msg.size()==N);
    std::vector<std::vector<int>> matt(q,std::vector<int>(q));
    int log2q=0;{int tmp=q;while(tmp>>=1)++log2q;}

    if (d <= 20 * log2q) {
        auto nc=newton2D(msg,d,q);
        std::vector<long long> inv_fact(d+1);
        inv_fact[0]=1;
        {long long f=1;for(int i=1;i<=d;++i)f=f*i%q;inv_fact[d]=mpow(f,q-2,q);}
        for(int i=d-1;i>=1;--i)inv_fact[i]=inv_fact[i+1]*(i+1)%q;

        parallel_for(q,[&](int y){
            std::vector<long long> Cy(d+1);
            Cy[0]=1; long long fall=1;
            for(int r=1;r<=d;++r){fall=fall*((y-r+1+q)%q)%q;Cy[r]=fall*inv_fact[r]%q;}
            std::vector<long long> A(d+1,0);
            for(int p=0;p<=d;++p)for(int r=0;r<=d-p;++r)A[p]=(A[p]+nc[r][p]*Cy[r])%q;
            for(int x=0;x<q;++x){matt[x][y]=(int)A[0];for(int k=0;k<d;++k){A[k]+=A[k+1];if(A[k]>=(long long)q)A[k]-=q;}}
        });
    } else {
        auto nc=newton2D(msg,d,q);
        NTTState st=buildNTT(q,d);
        std::vector<std::vector<int>> T(d+1,std::vector<int>(q));
        parallel_for(d+1,[&](int p){
            int l=d-p+1; std::vector<long long> Ap(l);
            for(int r=0;r<l;++r)Ap[r]=nc[r][p];
            evalNTT(Ap,l-1,st,T[p]);
        });
        parallel_for(q,[&](int y){
            std::vector<long long> Ay(d+1); for(int p=0;p<=d;++p)Ay[p]=T[p][y];
            std::vector<int> row(q); evalNTT(Ay,d,st,row);
            for(int x=0;x<q;++x)matt[x][y]=row[x];
        });
    }
    (void)matt;
    return (long long)q*q*2;  // codeword bytes (2 bytes per element)
}

// ─── Benchmark runner ─────────────────────────────────────────────────────

int main() {
    int nthreads = hw_concurrency();
    printf("Threads: %d\n\n", nthreads);
    printf("%-60s  %8s  %12s\n", "Config", "Time(s)", "Codeword");
    printf("%s\n", std::string(85, '-').c_str());

    struct P { int q, d, k; bool lrs; const char* label; };
    // M for LRS = d² - max(0, 2d-q).  For minimal q ≈ k*d with k≥4, q > 2d so M = d².
    std::vector<P> cases = {
        // ── RM, minimal q, k=4 ────────────────────────────────────────────
        {   19,  4, 4,false,"RM  q=19    M=15    d=4   k=4  ≈722B  "},
        {  211, 50, 4,false,"RM  q=211   M=1326  d=50  k=4  ≈87KB  "},
        {  409,100, 4,false,"RM  q=409   M=5151  d=100 k=4  ≈327KB "},
        {  809,200, 4,false,"RM  q=809   M=20301 d=200 k=4  ≈1.3MB "},
        { 1123,280, 4,false,"RM  q=1123  M=39621 d=280 k=4  ≈2.5MB "},
        { 1607,400, 4,false,"RM  q=1607  M=80601 d=400 k=4  ≈5.2MB "},
        // ── LRS, same q and d, k=4  (M_lrs = d², ~2× larger database) ───
        {   19,  4, 4,true, "LRS q=19    M=16    d=4   k=4  ≈722B  "},
        {  211, 50, 4,true, "LRS q=211   M=2500  d=50  k=4  ≈87KB  "},
        {  409,100, 4,true, "LRS q=409   M=10000 d=100 k=4  ≈327KB "},
        {  809,200, 4,true, "LRS q=809   M=40000 d=200 k=4  ≈1.3MB "},
        { 1123,280, 4,true, "LRS q=1123  M=78120 d=280 k=4  ≈2.5MB "},
        { 1607,400, 4,true, "LRS q=1607  M=160000d=400 k=4  ≈5.2MB "},
        // ── RM, minimal q, k=5 ────────────────────────────────────────────
        {  503,100, 5,false,"RM  q=503   M=5151  d=100 k=5  ≈494KB "},
        { 1009,200, 5,false,"RM  q=1009  M=20301 d=200 k=5  ≈1.9MB "},
        { 2003,400, 5,false,"RM  q=2003  M=80601 d=400 k=5  ≈7.7MB "},
        // ── LRS, same q and d, k=5 ───────────────────────────────────────
        {  503,100, 5,true, "LRS q=503   M=10000 d=100 k=5  ≈494KB "},
        { 1009,200, 5,true, "LRS q=1009  M=40000 d=200 k=5  ≈1.9MB "},
        { 2003,400, 5,true, "LRS q=2003  M=160000d=400 k=5  ≈7.7MB "},
    };

    std::mt19937 rng(42);
    for (auto& p : cases) {
        if (p.d * p.k + 1 >= p.q) { printf("%-60s  SKIP\n", p.label); continue; }

        int N = p.lrs ? (int)goodMonomials2D(p.q, p.d).size()
                      : binomial(p.d + 2, 2);
        std::vector<int> msg(N);
        std::uniform_int_distribution<int> dist(0, p.q - 1);
        for (auto& x : msg) x = dist(rng);

        auto encode = [&]() -> long long {
            return p.lrs ? LRSE2D(msg, p.q, p.d) : RME2D(msg, p.q, p.d);
        };

        // Warmup
        encode();

        // Timed run
        auto t0 = std::chrono::high_resolution_clock::now();
        long long bytes = encode();
        auto t1 = std::chrono::high_resolution_clock::now();
        double s = std::chrono::duration<double>(t1 - t0).count();

        double mb = bytes / 1e6;
        char size_str[32];
        if (mb < 1000) snprintf(size_str, sizeof(size_str), "%.0fMB", mb);
        else snprintf(size_str, sizeof(size_str), "%.1fGB", mb/1000);

        printf("%-60s  %8.3f  %12s\n", p.label, s, size_str);
        fflush(stdout);
    }
    return 0;
}
