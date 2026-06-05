"""
Encoding benchmark: time vs DB size (before encoding), minimal-q strategy.
Run after bench_encode_st and bench_encode complete.
"""
import matplotlib.pyplot as plt
import matplotlib.ticker as ticker
import numpy as np

# ── Data (M, d, k, q, db_bytes, time_1t_s, time_15t_s) ──────────────────────
# db_bytes = M * 2  (2 bytes per F_q element, q < 65536)
# times marked None are still pending (ST benchmark running)

data = [
    # M,     d,   k,  q,     db_bytes,   st_s,    mt_s
    (15,      4,  4,  19,    30,         0.0001,   0.0001),
    (1326,   50,  4,  211,   2652,       0.001,    0.0001),
    (5151,  100,  4,  409,   10302,      0.007,    0.001),
    (20301, 200,  4,  809,   40602,      0.167,    0.025),
    (39621, 280,  4,  1123,  79242,      0.514,    0.058),
    (45451, 300,  4,  1213,  90902,      0.561,    0.063),
    (80601, 400,  4,  1607,  161202,     0.759,    0.091),

    (15,      4,  5,  23,    30,         0.0001,   0.0001),
    (1326,   50,  5,  257,   2652,       0.001,    0.0001),
    (5151,  100,  5,  503,   10302,      0.009,    0.002),
    (20301, 200,  5,  1009,  40602,      0.436,    0.050),
    (39621, 280,  5,  1409,  79242,      0.617,    0.069),
    (45451, 300,  5,  1511,  90902,      0.661,    0.072),
    (80601, 400,  5,  2003,  161202,     1.927,    0.198),
]

# ── Extrapolation to ~1 GB input ─────────────────────────────────────────────
# 1 GB = 2^30 bytes, 2 bytes/elem → M ≈ 2^29 ≈ 537M
# C(d+2,2) ≈ d²/2 → d ≈ sqrt(2 * 537M) ≈ 32,749 → use d = 32768
# k=4: q_min > 4*32768+1 = 131073, q ≈ 131101 (next prime)
# k=5: q_min > 5*32768+1 = 163841, q ≈ 163841 (next prime)
GB = 1 * 1024**3
d_1g = 32768
M_1g = d_1g * (d_1g + 1) // 2  # C(d+2,2) ≈ d²/2
db_1g = M_1g * 2

# Extrapolate from d=400 NTT measurements using O(q²·log(N)) scaling
# where N = next_pow2(d+q+1), q ≈ k*d for minimal-q
def ntt_cost(d, k):
    q = k * d
    N = 1
    while N < d + q + 1:
        N <<= 1
    return q * N * np.log2(N)   # phase 2 dominates

scale_k4 = ntt_cost(d_1g, 4) / ntt_cost(400, 4)
scale_k5 = ntt_cost(d_1g, 5) / ntt_cost(400, 5)
est_k4_mt = 0.091 * scale_k4   # seconds, 15 threads
est_k5_mt = 0.198 * scale_k5

# ── Separate by k ────────────────────────────────────────────────────────────
k4_st = [(r[4], r[5]) for r in data if r[2] == 4]
k4_mt = [(r[4], r[6]) for r in data if r[2] == 4]
k5_st = [(r[4], r[5]) for r in data if r[2] == 5]
k5_mt = [(r[4], r[6]) for r in data if r[2] == 5]

fig, ax = plt.subplots(figsize=(10, 6))

ax.loglog(*zip(*k4_st), 'o--', color='steelblue',  alpha=0.5, label='k=4, 1-thread')
ax.loglog(*zip(*k4_mt), 'o-',  color='steelblue',  label='k=4, 15-thread')
ax.loglog(*zip(*k5_st), 's--', color='darkorange', alpha=0.5, label='k=5, 1-thread')
ax.loglog(*zip(*k5_mt), 's-',  color='darkorange', label='k=5, 15-thread')

# Extrapolation points (open markers)
ax.loglog([db_1g], [est_k4_mt], 'o', color='steelblue',
          markersize=13, markerfacecolor='none', markeredgewidth=2)
ax.loglog([db_1g], [est_k5_mt], 's', color='darkorange',
          markersize=13, markerfacecolor='none', markeredgewidth=2)

# Annotation for 1 GB
for est, k, dy in [(est_k4_mt, 4, 30), (est_k5_mt, 5, -45)]:
    mins = est / 60
    ax.annotate(f'~1 GB input\nk={k}: ~{mins:.0f} min (15T)',
                xy=(db_1g, est),
                xytext=(-100, dy),
                textcoords='offset points',
                fontsize=8,
                arrowprops=dict(arrowstyle='->', color='gray'))

# Reference lines
for t_label, t_val in [('1s', 1), ('1min', 60), ('10min', 600)]:
    ax.axhline(t_val, color='gray', linestyle='--', linewidth=0.6, alpha=0.5)
    ax.text(35, t_val * 1.15, t_label, fontsize=7, color='gray')

ax.set_xlabel('Database size before encoding (bytes)', fontsize=11)
ax.set_ylabel('Encoding time (seconds)', fontsize=11)
ax.set_title('RM Encoding: time vs DB size (minimal-q, 15 threads)\n'
             'open markers = extrapolated 1 GB estimate', fontsize=11)
ax.legend(fontsize=10)
ax.xaxis.set_major_formatter(ticker.FuncFormatter(
    lambda x, _: f'{x:.0f}B' if x < 1e3 else
                  f'{x/1e3:.0f}KB' if x < 1e6 else
                  f'{x/1e6:.0f}MB' if x < 1e9 else
                  f'{x/1e9:.1f}GB'))
ax.grid(True, which='both', linestyle=':', alpha=0.4)
plt.tight_layout()
plt.savefig('encoding_time.png', dpi=150)
print(f'Saved encoding_time.png')
print(f'\n1 GB input (d={d_1g}, M={M_1g:,}):')
print(f'  k=4: q≈131,101, codeword≈{131101**2*2/1e9:.1f} GB, est {est_k4_mt/60:.1f} min (15T)')
print(f'  k=5: q≈163,841, codeword≈{163841**2*2/1e9:.1f} GB, est {est_k5_mt/60:.1f} min (15T)')
