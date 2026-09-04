import glob,json,os,collections,statistics,math
from datetime import datetime
files=sorted(glob.glob('backend/data/cache/t0/selection/t0_selection_*.json'))
rows=[]; archives=[]
for f in files:
    try: a=json.load(open(f))
    except Exception as e: print('BAD',f,e); continue
    d=a.get('date') or os.path.basename(f)[13:-5]
    valid=0
    for r in a.get('results',[]):
        o=r.get('T0开盘涨幅(%)'); c=r.get('T0收盘涨幅(%)')
        if isinstance(o,(int,float)) and isinstance(c,(int,float)) and not (o==0 and c==0):
            x=dict(r);x['_date']=d;x['_pnl']=round(c-o,4); rows.append(x);valid+=1
    archives.append((d,a.get('count',0),valid, bool(a.get('close_updated_at'))))
print('files',len(files),'archive dates',archives[0][0],archives[-1][0],'raw count',sum(x[1] for x in archives),'valid pnl rows',len(rows),'dates valid',sum(1 for x in archives if x[2]))
print('field counts',collections.Counter(k for r in rows for k in r).most_common())
def stat(rs):
 p=[r['_pnl'] for r in rs];n=len(p); pos=[x for x in p if x>0]; neg=[x for x in p if x<0]
 return dict(n=n, win=sum(x>0 for x in p)/n*100 if n else 0,flat=sum(x==0 for x in p)/n*100 if n else 0,mean=sum(p)/n if n else 0,med=statistics.median(p) if n else 0,p25=statistics.quantiles(p,n=4,method='inclusive')[0] if n>1 else None,p75=statistics.quantiles(p,n=4,method='inclusive')[2] if n>1 else None,profit_avg=sum(pos)/len(pos) if pos else None,loss_avg=sum(neg)/len(neg) if neg else None,gt4=sum(x>=4 for x in p)/n*100 if n else 0,ltm4=sum(x<=-4 for x in p)/n*100 if n else 0,min=min(p)if n else None,max=max(p)if n else None)
print('ALL',stat(rows))
for threshold in [0,30,40,45,50,55,60,65,70,75,80,90]:
 rs=[r for r in rows if isinstance(r.get('形态达标率(%)'),(int,float)) and r['形态达标率(%)']>threshold]
 print('threshold>',threshold,stat(rs))
for sig in sorted(set(r.get('买入信号','') for r in rows)):
 rs=[r for r in rows if r.get('买入信号','')==sig]; print('sig',sig,stat(rs))
print('tag')
for k in sorted(set(r.get('标记','') for r in rows)):
 rs=[r for r in rows if r.get('标记','')==k]; print(k or '无标记',stat(rs))
print('top patterns w>70')
groups=collections.defaultdict(list)
for r in rows:
 if isinstance(r.get('形态达标率(%)'),(int,float)) and r['形态达标率(%)']>70:groups[r.get('形态','')].append(r)
for p,rs in sorted(groups.items(),key=lambda kv:(-len(kv[1]),kv[0])):
 print(p, 'recorded',rs[0].get('形态达标率(%)'), 't0n',rs[0].get('形态样本数'), stat(rs), 'date',min(r['_date'] for r in rs),max(r['_date'] for r in rs))
