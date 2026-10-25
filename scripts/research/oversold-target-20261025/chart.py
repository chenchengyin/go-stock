from pathlib import Path
import json
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
from matplotlib.font_manager import FontProperties
import numpy as np
import pandas as pd

OUT=Path(__file__).resolve().parents[3]/'docs/research/2026-10-25-oversold-target-optimization'
font=Path('/System/Library/Fonts/Hiragino Sans GB.ttc')
if font.exists():plt.rcParams['font.family']=FontProperties(fname=str(font)).get_name()
plt.rcParams['axes.unicode_minus']=False;plt.rcParams['font.size']=11
r=pd.read_csv(OUT/'evaluation.csv')
def get(name,period):return r[(r.rule==name)&(r.period==period)].iloc[0]
fig,axs=plt.subplots(1,2,figsize=(12.5,5.6),layout='constrained')
ax=axs[0];x=np.arange(3);w=.25
for shift,field,label,color in [(-w,'day_target_pct','F1：按日等权','#287f8e'),(0,'remove3_day_target_pct','F1：剔除最多信号3日','#93b6bd')]:
 vals=[get('F1',p)[field] for p in ['2024','2025','2026']]
 ax.bar(x+shift,vals,w,label=label,color=color)
 for xx,v in zip(x+shift,vals):ax.text(xx,v+.6,f'{v:.1f}',ha='center',fontsize=9)
vals=[get('prev10',p).day_target_pct for p in ['2024','2025','2026']]
ax.bar(x+w,vals,w,label='仅前日七日跌幅≥10%',color='#bec4ca')
for xx,v in zip(x+w,vals):ax.text(xx,v+.6,f'{v:.1f}',ha='center',fontsize=9)
ax.set_xticks(x,['2024\n发现条件','2025\n筛选验证','2026\n冻结后复核'])
ax.set_ylim(0,50);ax.set_ylabel('次日收盘净收益>2.5%的比例（%）');ax.set_title('改善没有消失，但后段达标率仍不高')
ax.legend(loc='upper right',fontsize=9,frameon=False)
ax=axs[1];ax.axvspan(150,700,color='#edf4f2');ax.axvline(150,ls='--',color='#7b9c91',lw=1)
labels=[('old_D6_common_universe','旧6%版',(-2,13),'#be6654'),('old_D4_common_universe','旧4%版',(8,-19),'#d99155'),('F1','冻结候选F1',(12,3),'#287f8e'),('prev10','宽松超跌基线',(-120,12),'#87929c')]
for rule,label,offset,color in labels:
 z=get(rule,'all');ax.scatter(z.days,z.day_target_pct,s=90,color=color,zorder=3)
 ax.annotate(f'{label}\n{int(z.n)}笔 / {int(z.days)}日\n{z.day_target_pct:.1f}%',(z.days,z.day_target_pct),xytext=offset,textcoords='offset points',fontsize=10,color=color)
ax.text(380,53,'至少150个不同信号日的区域',ha='center',fontsize=10,color='#52776b')
ax.set_xlim(0,700);ax.set_ylim(0,60);ax.set_xlabel('不同信号日期数量');ax.set_ylabel('按日等权的达标比例（%）');ax.set_title('较高比例的旧版，仍缺足够信号日期')
for ax in axs:ax.spines[['top','right']].set_visible(False);ax.grid(axis='y',alpha=.15);ax.set_axisbelow(True)
fig.suptitle('固定目标：尾盘收盘价代理入场 → 次日收盘净收益严格超过2.5%',fontsize=14,fontweight='bold')
fig.savefig(OUT/'optimization-summary.png',dpi=180);fig.savefig(OUT/'optimization-summary.svg')
print(OUT/'optimization-summary.png')
