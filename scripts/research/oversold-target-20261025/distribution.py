"""Empirical net-return distributions, with Gaussian fits only as references."""
from pathlib import Path
import json
import math
import numpy as np
import pandas as pd
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
from matplotlib.font_manager import FontProperties

ROOT=Path(__file__).resolve().parents[3]
OUT=ROOT/'docs/research/2026-10-25-return-distribution'
OUT.mkdir(parents=True,exist_ok=True)
SOURCES={
    'old_D4':('旧D4组合',ROOT/'docs/research/2026-10-24-oversold-close/candidate-minus4-samples.csv'),
    'new_F1':('新F1组合',ROOT/'docs/research/2026-10-25-oversold-target-optimization/F1-selected-samples.csv'),
}
COLORS=['#c66c66','#d9ac55','#338f80']
font=Path('/System/Library/Fonts/Hiragino Sans GB.ttc')
if font.exists():plt.rcParams['font.family']=FontProperties(fname=str(font)).get_name()
plt.rcParams['axes.unicode_minus']=False;plt.rcParams['font.size']=11


def summarize(x,w):
    a=x.net.to_numpy();w=np.asarray(w,float);w=w/w.sum();mu=float(np.sum(w*a))
    sd=float(np.sqrt(np.sum(w*(a-mu)**2)));z=(a-mu)/sd
    return {
        'n':len(x),'dates':int(x.date.nunique()),'mean_net_pct':mu,'std_population_pct':sd,
        'next_yang_pct':float(100*np.sum(w*x.n_close.gt(x.n_open).to_numpy())),
        'next_red_vs_prev_close_pct':float(100*np.sum(w*x.n_close.gt(x.close).to_numpy())),
        'net_positive_pct':float(100*np.sum(w*(a>1e-9))),
        'nonpositive_pct':float(100*np.sum(w*(a<=1e-9))),
        'net_0_to_2_5_pct':float(100*np.sum(w*((a>1e-9)&(a<=2.5+1e-9)))),
        'net_gt2_5_pct':float(100*np.sum(w*(a>2.5+1e-9))),
        'normal_fit_gt2_5_pct':50*math.erfc((2.5-mu)/(sd*math.sqrt(2))),
        'skew_population':float(np.sum(w*z**3)),
        'excess_kurtosis_population':float(np.sum(w*z**4)-3),
    }


def hist_plot(datasets,mode,filename):
    fig,axs=plt.subplots(1,2,figsize=(13,5.7),layout='constrained',sharex=True,sharey=True)
    edges=np.arange(-11,11.0001,.5);width=.5;grid=np.linspace(-11,11,700)
    for ax,(key,label,d) in zip(axs,datasets):
        w=np.ones(len(d))/len(d) if mode=='stock' else 1/(d.groupby('date').net.transform('size').to_numpy()*d.date.nunique())
        ss=summarize(d,w);values,_=np.histogram(d.net,bins=edges,weights=w*100)
        mids=(edges[1:]+edges[:-1])/2
        colors=[COLORS[0] if v<0 else COLORS[1] if v<2.5 else COLORS[2] for v in mids]
        ax.bar(mids,values,width=.47,color=colors,alpha=.85,linewidth=.3,edgecolor='white',label='实际收益分布')
        mu,sd=ss['mean_net_pct'],ss['std_population_pct']
        pdf=np.exp(-.5*((grid-mu)/sd)**2)/(sd*math.sqrt(2*math.pi))
        ax.plot(grid,pdf*width*100,'--',color='#343b45',lw=1.8,label='按均值/标准差拟合正态曲线')
        ax.axvline(0,color='#9c514e',lw=1.2);ax.axvline(2.5,color='#237c6b',lw=1.3)
        ax.set_title(f'{label}：{len(d)}个股票日 / {d.date.nunique()}个信号日',pad=12)
        ax.set_xlim(-11,11);ax.set_xticks([-10,-7.5,-5,-2.5,0,2.5,5,7.5,10])
        ax.set_xlabel('买入收盘价 → 次日收盘的净收益（%）')
        ax.text(.03,.96,f'亏损或持平：{ss["nonpositive_pct"]:.1f}%\n净赚0～2.5%：{ss["net_0_to_2_5_pct"]:.1f}%\n净赚>2.5%：{ss["net_gt2_5_pct"]:.1f}%',transform=ax.transAxes,va='top',fontsize=11,bbox={'facecolor':'white','edgecolor':'#dce2e5','alpha':.95,'boxstyle':'round,pad=.5'})
        ax.text(.97,.96,f'均值 {mu:+.2f}%\n标准差 {sd:.2f}个百分点',transform=ax.transAxes,ha='right',va='top',fontsize=10)
        ax.set_ylim(0,20);ax.spines[['top','right']].set_visible(False);ax.grid(axis='y',alpha=.15);ax.set_axisbelow(True)
        ax.legend(loc='upper center',bbox_to_anchor=(.5,-.19),frameon=False,fontsize=9)
    axs[0].set_ylabel('每0.5个百分点收益区间的样本占比（%）')
    heading='股票日等权（对应此前逐笔比例）' if mode=='stock' else '信号日等权（每个日期总权重相同）'
    fig.suptitle(f'实际净收益分布与正态拟合参考\n{heading}；扣除假设总成本0.15%',fontsize=14,fontweight='bold')
    fig.savefig(OUT/filename,dpi=180);fig.savefig(OUT/filename.replace('.png','.svg'))
    plt.close(fig)


def main():
    datasets=[];summaries=[];binned=[];details=[]
    bounds=[-np.inf,-7.5,-5,-2.5,0,1,2.5,5,7.5,np.inf]
    labels=['≤−7.5%','(−7.5%,−5%]','(−5%,−2.5%]','(−2.5%,0%]','(0%,1%]','(1%,2.5%]','(2.5%,5%]','(5%,7.5%]','>7.5%']
    for key,(label,path) in SOURCES.items():
        d=pd.read_csv(path,dtype={'code':str});d['net']=100*(d.n_close/d.close-1)-.15
        assert not d[['code','date']].duplicated().any()
        datasets.append((key,label,d))
        for period,x in [('all',d),('2026',d[d.date.str.startswith('2026')])]:
            for mode in ['stock','day']:
                w=np.ones(len(x))/len(x) if mode=='stock' else 1/(x.groupby('date').net.transform('size').to_numpy()*x.date.nunique())
                ss=summarize(x,w);ss.update({'rule':key,'label':label,'period':period,'weighting':mode})
                if mode=='stock':
                    ss['quantiles']={str(q):float(x.net.quantile(q)) for q in [0,.05,.1,.25,.5,.75,.9,.95,1]}
                    ss['net_positive_n']=int(x.net.gt(1e-9).sum());ss['net_gt2_5_n']=int(x.net.gt(2.5+1e-9).sum())
                    ss['net_0_to_2_5_n']=int((x.net.gt(1e-9)&x.net.le(2.5+1e-9)).sum())
                    ss['nonpositive_n']=len(x)-ss['net_positive_n']
                summaries.append(ss)
            # Fixed bin edges, right-closed, aligned with the strict target definition.
            cats=pd.cut(x.net,bounds,labels=labels,right=True)
            for label_bin in labels:
                mask=cats.eq(label_bin);n=int(mask.sum());binned.append({'rule':key,'period':period,'net_return_interval':label_bin,'n':n,'pct':100*n/len(x)})
            assert sum(z['n'] for z in binned if z['rule']==key and z['period']==period)==len(x)
        selected=d[['code','name','date','close','n_date','n_open','n_close','net']].copy();selected['rule']=key;details.append(selected)
    (OUT/'distribution-summary.json').write_text(json.dumps(summaries,ensure_ascii=False,indent=2))
    pd.DataFrame([{k:v for k,v in a.items() if k!='quantiles'} for a in summaries]).to_csv(OUT/'distribution-summary.csv',index=False)
    pd.DataFrame(binned).to_csv(OUT/'收益分段统计.csv',index=False)
    pd.concat(details).to_csv(OUT/'逐笔收益明细.csv',index=False)
    hist_plot(datasets,'stock','实际净收益分布.png')
    hist_plot(datasets,'day','日期等权收益分布.png')
    for ss in summaries:
        assert abs(ss['nonpositive_pct']+ss['net_0_to_2_5_pct']+ss['net_gt2_5_pct']-100)<1e-8
    old=next(x for x in summaries if x['rule']=='old_D4' and x['period']=='all' and x['weighting']=='stock')
    new=next(x for x in summaries if x['rule']=='new_F1' and x['period']=='all' and x['weighting']=='stock')
    assert old['n']==477 and old['net_gt2_5_n']==315 and new['n']==818 and new['net_gt2_5_n']==255
    assert new['nonpositive_n']==418 and new['net_0_to_2_5_n']==145
    print(pd.DataFrame(summaries)[['rule','period','weighting','n','dates','next_yang_pct','next_red_vs_prev_close_pct','net_positive_pct','net_gt2_5_pct','mean_net_pct','std_population_pct','normal_fit_gt2_5_pct']].to_string(index=False))
    print(pd.DataFrame(binned).query("period=='all'").to_string(index=False))
    print('Verified counts, partition totals and original target counts; figures saved to',OUT)


if __name__=='__main__':main()
