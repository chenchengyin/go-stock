from pathlib import Path
import json
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.font_manager import FontProperties
import numpy as np

out=Path(__file__).resolve().parents[3]/"docs/research/2026-10-24-oversold-close"
rows=json.loads((out/"robustness.json").read_text())
font=Path("/System/Library/Fonts/Hiragino Sans GB.ttc")
if font.exists():
    plt.rcParams["font.family"]=FontProperties(fname=str(font)).get_name()
plt.rcParams["axes.unicode_minus"]=False
plt.rcParams["font.size"]=11
fig,axs=plt.subplots(1,2,figsize=(12.8,5.5),layout="constrained")
for ax,period,title in zip(axs,["all","2026"],["2024—2026 年样本","2026 年样本"]):
    r=next(x for x in rows if x['rule']=='D_body_lt_-4' and x['period']==period)
    top=3 if period=='all' else 1
    removed=r['remove_top_signal_dates'][str(top)]
    vals=[r['yang_pct'],r['equal_day_yang_pct'],removed['yang_pct'],r['one_stock_per_day']['amount_yi']['yang_pct']]
    labels=['每个股票日\n同等权重','每个信号日\n同等权重',f'剔除信号最多的\n{top} 个交易日','每天只选\n成交额最大一只']
    ax.bar(np.arange(4),vals,color=['#287f8e','#879ea9','#d99155','#be6654'],width=.64)
    for i,v in enumerate(vals):ax.text(i,v+1.4,f'{v:.1f}%',ha='center',fontweight='bold')
    ax.axhline(50,color='#8d959c',ls='--',lw=1)
    ax.set_ylim(0,103);ax.set_xticks(np.arange(4),labels);ax.set_ylabel('次日收阳比例（%）')
    ax.set_title(f'{title}\n{r["n"]} 个股票日，{r["days"]} 个信号日',pad=16)
    ax.spines[['top','right']].set_visible(False);ax.grid(axis='y',alpha=.15);ax.set_axisbelow(True)
fig.suptitle('“历史收阳率 81.6%”为什么还不能证明尾盘策略稳定？',fontsize=16,fontweight='bold')
fig.savefig(out/'concentration.png',dpi=180)
fig.savefig(out/'concentration.svg')
print(out/'concentration.png')
