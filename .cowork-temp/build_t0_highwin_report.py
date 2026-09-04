import json, os, math, statistics, collections
from datetime import datetime

SRC='.cowork-temp/t0-observations-2025-2026.json'
OUT='T0主板策略_高胜率形态浮盈分析.html'
data=json.load(open(SRC, encoding='utf-8'))
rows=data['rows']

def percentile(xs,p):
    xs=sorted(xs)
    if not xs:return 0
    k=(len(xs)-1)*p; f=math.floor(k); c=math.ceil(k)
    return xs[f] if f==c else xs[f]*(c-k)+xs[c]*(k-f)

def wilson(k,n,z=1.96):
    if n==0:return (0,0)
    ph=k/n; den=1+z*z/n; cen=(ph+z*z/(2*n))/den
    half=z*math.sqrt((ph*(1-ph)+z*z/(4*n))/n)/den
    return cen*100-half*100, cen*100+half*100

def summary(xs):
    n=len(xs); w=sum(x>=2.5 for x in xs); pos=sum(x>0 for x in xs); neg=sum(x<0 for x in xs)
    return {
        'n':n,'ach':w,'achRate':100*w/n if n else 0,'pos':pos,'posRate':100*pos/n if n else 0,
        'neg':neg,'mean':sum(xs)/n if n else 0,'median':statistics.median(xs) if xs else 0,
        'q1':percentile(xs,.25),'q3':percentile(xs,.75),'min':min(xs) if xs else 0,'max':max(xs) if xs else 0,
        'ci':wilson(w,n), 'profitMean':statistics.mean([x for x in xs if x>0]) if pos else 0,
        'lossMean':statistics.mean([x for x in xs if x<0]) if neg else 0,
    }

groups=collections.defaultdict(list)
for r in rows: groups[r['pattern']].append(r)
pattern_stats=[]
for p,rs in groups.items():
    ss=summary([r['pnl'] for r in rs])
    if ss['achRate']>70:
        pattern_stats.append({'pattern':p,'rows':rs,**ss})
pattern_stats.sort(key=lambda x:(-x['n'],-x['achRate'], -x['mean']))
qualified=[x for x in pattern_stats if x['n']>=5]
selected_rows=sorted([r for x in qualified for r in x['rows']], key=lambda r:(r['date'],r['pattern']))
S=summary([r['pnl'] for r in selected_rows])
ALL=summary([r['pnl'] for r in rows])

# quality ladder
def ladder(min_n):
 ps=[p for p in pattern_stats if p['n']>=min_n]
 ev=[r for p in ps for r in p['rows']]
 return {'minN':min_n,'patterns':len(ps),'n':len(ev),'rate':summary([r['pnl'] for r in ev])['achRate'] if ev else 0}
ladder=[ladder(x) for x in (1,3,5,7,10)]

# histogram high selected
bins=[(-1e9,-5,'≤ −5%'),(-5,-2.5,'−5% ~ −2.5%'),(-2.5,0,'−2.5% ~ 0%'),(0,2.5,'0% ~ 2.5%'),(2.5,5,'2.5% ~ 5%'),(5,7.5,'5% ~ 7.5%'),(7.5,1e9,'≥ 7.5%')]
hist=[]
for lo,hi,label in bins:
    vals=[r for r in selected_rows if lo <= r['pnl'] < hi]
    hist.append({'label':label,'count':len(vals),'pnl':sum(r['pnl'] for r in vals)})

# all date sample membership high patterns
patColors={'DYIN|YX|MYIN':'#3ee0a7','ZT|DYIN|DT':'#65a7ff'}
for r in selected_rows: r['color']=patColors.get(r['pattern'],'#ddd')

# values for json
payload={'meta':{'range':f"{data['date_start']} ~ {data['date_end']}",'tradingDays':data['trading_days'],'allN':len(rows)},'all':ALL,'selected':S,'qualified':[{k:v for k,v in x.items() if k!='rows'} for x in qualified],'allHigh':[{k:v for k,v in x.items() if k!='rows'} for x in pattern_stats if x['n']>=3], 'ladder':ladder,'hist':hist,'trades':selected_rows}
json_payload=json.dumps(payload,ensure_ascii=False,separators=(',',':'))

def f(v,d=2): return f'{v:.{d}f}'
def signed(v): return ('+' if v>0 else '')+f(v)+'%'

cards=[
 ('筛选后形态','2 个','历史达标率 >70% 且 T0 样本 ≥5'),
 ('复盘样本',f"{S['n']} 笔",f"{data['trading_days']} 个交易日的主板 T0 归档重算"),
 ('达标胜率',f"{f(S['achRate'],1)}%",'项目定义：开盘至收盘浮盈 ≥2.5%'),
 ('平均浮盈',signed(S['mean']),f"中位数 {signed(S['median'])}"),
 ('正浮盈占比',f"{f(S['posRate'],1)}%",f"{S['pos']} 正 / {S['neg']} 负"),
]
cards_html=''.join(f'<section class="metric"><div>{a}</div><strong>{b}</strong><small>{c}</small></section>' for a,b,c in cards)
pattern_rows=''.join(f'''<tr><td><i class="dot" style="background:{patColors.get(p['pattern'],'#fff')}"></i><code>{p['pattern']}</code></td><td>{p['n']}</td><td class="good">{f(p['achRate'],1)}%</td><td>{f(p['posRate'],1)}%</td><td class="good">{signed(p['mean'])}</td><td>{signed(p['median'])}</td><td>{f(p['ci'][0],1)}%–{f(p['ci'][1],1)}%</td></tr>''' for p in qualified)
trade_rows=''.join(f'''<tr><td>{r['date']}</td><td><code>{r['pattern']}</code></td><td>{f(r['open_gap'])}%</td><td class="{'good' if r['pnl']>=2.5 else ('loss' if r['pnl']<0 else '')}">{signed(r['pnl'])}</td><td>{'达标' if r['pnl']>=2.5 else ('未达标' if r['pnl']>=0 else '浮亏')}</td></tr>''' for r in selected_rows)
ladder_rows=''.join(f'<tr><td>≥ {r["minN"]}</td><td>{r["patterns"]}</td><td>{r["n"]}</td><td>{f(r["rate"],1)}%</td></tr>' for r in ladder)

html=f'''<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>主板策略｜高胜率形态 T0 浮盈复盘</title>
<style>
:root{{--bg:#101418;--panel:#182027;--panel2:#1e2932;--ink:#edf1eb;--muted:#9ba9ad;--line:#2d3a42;--mint:#3ee0a7;--blue:#65a7ff;--red:#ff7171;--gold:#f3bf54}}*{{box-sizing:border-box}}body{{margin:0;background:radial-gradient(circle at 5% 0,#273540 0,transparent 28rem),var(--bg);color:var(--ink);font-family:"Noto Serif CJK SC","Songti SC",STSong,serif;letter-spacing:.015em}}.wrap{{max-width:1460px;margin:auto;padding:52px 38px 80px}}.eyebrow{{font:700 12px ui-monospace,Menlo,monospace;letter-spacing:.16em;color:var(--mint);text-transform:uppercase}}h1{{font-size:clamp(34px,5vw,68px);font-weight:600;line-height:1.04;margin:12px 0 16px;letter-spacing:-.06em}}h1 em{{font-style:normal;color:var(--mint)}}.subtitle{{max-width:820px;color:#bdc8c8;font-size:16px;line-height:1.7}}.warning{{border-left:4px solid var(--gold);background:#2a271d;padding:15px 18px;margin:30px 0 36px;line-height:1.7;color:#f4e1b6}}.metrics{{display:grid;grid-template-columns:repeat(5,1fr);gap:12px;margin-bottom:26px}}.metric,.card{{background:linear-gradient(145deg,rgba(31,43,52,.96),rgba(19,27,33,.96));border:1px solid var(--line);box-shadow:0 20px 60px rgba(0,0,0,.14)}}.metric{{padding:19px;min-height:142px;display:flex;flex-direction:column;justify-content:space-between}}.metric div{{color:var(--muted);font-size:13px}}.metric strong{{font-size:31px;letter-spacing:-.04em}}.metric small{{font-size:12px;line-height:1.4;color:#aebabb}}.grid{{display:grid;grid-template-columns:1.25fr .75fr;gap:18px;margin:18px 0}}.card{{padding:24px;overflow:hidden}}h2{{font-size:18px;margin:0 0 8px;font-weight:600}}.note{{font-size:12px;color:var(--muted);line-height:1.6;margin:0 0 15px}}.chart{{width:100%;min-height:330px}}.section-title{{display:flex;justify-content:space-between;gap:16px;align-items:baseline;margin-top:38px}}.section-title span{{font:12px ui-monospace,Menlo,monospace;color:var(--muted)}}table{{width:100%;border-collapse:collapse;font-size:13px}}th{{color:#8fa0a3;font-weight:400;text-align:right;border-bottom:1px solid var(--line);padding:10px 8px}}td{{text-align:right;padding:11px 8px;border-bottom:1px solid rgba(45,58,66,.62)}}th:first-child,td:first-child{{text-align:left}}code{{font:12px ui-monospace,Menlo,monospace;color:#d7e6eb;background:#0f1519;padding:4px 5px}}.good{{color:var(--mint);font-weight:700}}.loss{{color:var(--red);font-weight:700}}.dot{{display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:7px}}.method{{display:grid;grid-template-columns:1fr 1fr 1fr;gap:18px;margin-top:18px}}.method div{{padding:18px;background:rgba(24,32,39,.75);border:1px solid var(--line);font-size:13px;line-height:1.7;color:#bcc9ca}}.method strong{{display:block;color:#fff;margin-bottom:5px}}footer{{margin-top:35px;border-top:1px solid var(--line);padding-top:17px;color:#809094;font:11px ui-monospace,Menlo,monospace;line-height:1.75}}@media(max-width:1000px){{.metrics{{grid-template-columns:repeat(2,1fr)}}.grid,.method{{grid-template-columns:1fr}}.wrap{{padding:34px 18px}}h1{{letter-spacing:-.04em}}.card{{padding:17px}}.table-wrap{{overflow:auto}}}}
</style></head><body><main class="wrap">
<div class="eyebrow">GO-STOCK / MAINBOARD STRATEGY / IN-SAMPLE REVIEW</div>
<h1>主板策略的<br><em>高胜率形态</em>，真的赚钱吗？</h1>
<p class="subtitle">以项目 T0 选股的主板过滤链为样本池，筛出「3K 形态历史达标率大于 70%」的组合，再观察其开盘买入、收盘卖出的日内浮盈分布。</p>
<div class="warning"><b>核心结论：</b>满足「达标率 &gt;70% 且样本数 ≥5」的形态只有 <b>2 个</b>，合计 <b>15 笔</b>。其历史达标胜率为 <b>{f(S['achRate'],1)}%</b>，平均日内浮盈 <b>{signed(S['mean'])}</b>；但没有任何形态达到项目生产信号要求的 <b>N≥10</b>，统计不确定性仍然很高，不宜直接当作实盘买入信号。</div>
<section class="metrics">{cards_html}</section>
<div class="grid"><section class="card"><h2>高胜率形态：达标率与平均浮盈</h2><p class="note">达标＝T0 浮盈 ≥2.5%。误差线为 95% Wilson 区间，样本很小，区间较宽。</p><svg id="patternChart" class="chart" viewBox="0 0 760 330" role="img"></svg></section><section class="card"><h2>15 笔样本的浮盈分布</h2><p class="note">横轴为开盘至收盘浮盈区间；绿线是 0%，金线是 +2.5% 达标线。</p><svg id="histChart" class="chart" viewBox="0 0 490 330" role="img"></svg></section></div>
<div class="grid"><section class="card"><h2>每笔 T0 日内浮盈轨迹</h2><p class="note">两种形态按发生日期排列。柱高为浮盈，颜色区分形态；横线为 +2.5% 达标线。</p><svg id="tradeChart" class="chart" viewBox="0 0 760 330" role="img"></svg></section><section class="card"><h2>样本门槛会改变答案</h2><p class="note">不设样本门槛时，许多 1–2 次命中的偶然形态也会显示 100% 胜率。</p><div class="table-wrap"><table><thead><tr><th>最小 T0 样本</th><th>形态数</th><th>覆盖笔数</th><th>合并达标率</th></tr></thead><tbody>{ladder_rows}</tbody></table></div><p class="note" style="margin-top:18px">全样本基准：{ALL['n']:,} 笔，达标率 {f(ALL['achRate'],1)}%，平均浮盈 {signed(ALL['mean'])}。当前高胜率分组相对基准很强，但属于同一历史样本内筛选，不是样本外验证。</p></section></div>
<div class="section-title"><h2>筛选出的形态</h2><span>WIN RATE &gt; 70% · N ≥ 5</span></div><section class="card table-wrap"><table><thead><tr><th>3K 形态</th><th>T0 样本</th><th>达标率</th><th>正浮盈率</th><th>平均浮盈</th><th>中位数</th><th>95% 胜率区间</th></tr></thead><tbody>{pattern_rows}</tbody></table></section>
<div class="section-title"><h2>逐笔复盘</h2><span>15 OBSERVATIONS / OPEN → CLOSE</span></div><section class="card table-wrap"><table><thead><tr><th>交易日</th><th>形态</th><th>开盘涨幅</th><th>日内浮盈</th><th>结果</th></tr></thead><tbody>{trade_rows}</tbody></table></section>
<section class="method"><div><strong>策略范围</strong>项目 T0 主板策略：60/00 开头，市值 50–9000 亿；近 7 日涨停记忆/前日破板；前日成交额 ≥5 亿；开盘涨幅 0.01%–3%。</div><div><strong>浮盈与胜率口径</strong>日内浮盈＝当日收盘涨幅－开盘涨幅。项目“形态达标率”定义为浮盈 ≥2.5%，与“正浮盈（&gt;0）”不是同一个指标。</div><div><strong>审慎解释</strong>形态统计与浮盈回看使用同一段历史，且高胜率形态没有达到系统配置的 N≥10。这里展示的是探索性复盘，不构成交易建议。</div></section>
<footer>数据来源：backend/data/cache/t0/daily/t0_daily_cache_*.gob ｜ 覆盖 {data['date_start']} 至 {data['date_end']} ｜ 生成时间：2026-09-04 10:30 Asia/Shanghai ｜ 计算口径与 backend/analysis/candlepattern/stats.go 对齐</footer>
</main><script>const D={json_payload};const $=s=>document.querySelector(s);const txt=(x,y,t,o={{}})=>`<text x="${{x}}" y="${{y}}" fill="${{o.fill||'#9ba9ad'}}" font-family="ui-monospace,Menlo,monospace" font-size="${{o.size||11}}" text-anchor="${{o.anchor||'start'}}">${{t}}</text>`;const line=(x1,y1,x2,y2,c='#33434c',w=1,d='')=>`<line x1="${{x1}}" y1="${{y1}}" x2="${{x2}}" y2="${{y2}}" stroke="${{c}}" stroke-width="${{w}}" stroke-dasharray="${{d}}"/>`;
(function(){{let w=760,h=330,p={{l:64,r:24,t:32,b:62}},cw=w-p.l-p.r,ch=h-p.t-p.b;let max=100;let s='';[0,25,50,75,100].forEach(v=>{{let y=p.t+ch*(1-v/max);s+=line(p.l,y,w-p.r,y);s+=txt(p.l-9,y+4,v+'%',{{anchor:'end'}})}});D.qualified.forEach((r,i)=>{{let x=p.l+cw*(i+.5)/D.qualified.length;let bh=ch*r.achRate/max;s+=`<rect x="${{x-42}}" y="${{p.t+ch-bh}}" width="84" height="${{bh}}" rx="3" fill="${{i?'#65a7ff':'#3ee0a7'}}"/>`;let lo=p.t+ch*(1-r.ci[0]/max),hi=p.t+ch*(1-r.ci[1]/max);s+=line(x,lo,x,hi,'#edf1eb',2);s+=line(x-7,lo,x+7,lo,'#edf1eb',2);s+=line(x-7,hi,x+7,hi,'#edf1eb',2);s+=txt(x,p.t+ch+23,r.pattern,{{anchor:'middle',fill:'#dce8e8',size:11}});s+=txt(x,p.t+ch+39,'N='+r.n+' · 均 '+(r.mean>0?'+':'')+r.mean.toFixed(2)+'%',{{anchor:'middle',size:10}})}});s+=txt(p.l,p.t-10,'达标率（柱） / 95%区间（误差线）',{{fill:'#dbe7e6'}});$('#patternChart').innerHTML=s}})();
(function(){{let w=490,h=330,p={{l:48,r:16,t:32,b:84}},cw=w-p.l-p.r,ch=h-p.t-p.b,max=Math.max(...D.hist.map(x=>x.count),1),s='';for(let v=0;v<=max;v++){{let y=p.t+ch*(1-v/max);s+=line(p.l,y,w-p.r,y);s+=txt(p.l-7,y+4,String(v),{{anchor:'end'}})}}D.hist.forEach((b,i)=>{{let bw=cw/D.hist.length-7,x=p.l+i*(cw/D.hist.length)+4,bh=b.count/max*ch,col=i<3?'#ff7171':i===3?'#8b999b':i===4?'#f3bf54':'#3ee0a7';s+=`<rect x="${{x}}" y="${{p.t+ch-bh}}" width="${{bw}}" height="${{bh}}" rx="3" fill="${{col}}"/>`;s+=txt(x+bw/2,p.t+ch-bh-7,b.count,{{anchor:'middle',fill:'#e9f0ed'}});let parts=b.label.split(' ');s+=txt(x+bw/2,p.t+ch+19,parts[0],{{anchor:'middle',size:9}});s+=txt(x+bw/2,p.t+ch+34,parts.slice(1).join(' '),{{anchor:'middle',size:9}})}});s+=txt(p.l,p.t-10,'笔数',{{fill:'#dbe7e6'}});$('#histChart').innerHTML=s}})();
(function(){{let w=760,h=330,p={{l:46,r:17,t:30,b:55}},cw=w-p.l-p.r,ch=h-p.t-p.b;let lo=-6,hi=10,y=v=>p.t+(hi-v)/(hi-lo)*ch,s='';[-5,0,2.5,5,10].forEach(v=>{{let yy=y(v),c=v===0?'#ff7171':v===2.5?'#f3bf54':'#33434c';s+=line(p.l,yy,w-p.r,yy,c,v===0||v===2.5?1.5:1,v===2.5?'4 3':'');s+=txt(p.l-7,yy+4,(v>0?'+':'')+v+'%',{{anchor:'end',fill:c}})}});let bw=Math.max(8,cw/D.trades.length*.58);D.trades.forEach((r,i)=>{{let x=p.l+cw*(i+.5)/D.trades.length,yy=y(r.pnl),y0=y(0),top=Math.min(yy,y0),hh=Math.abs(yy-y0);s+=`<rect x="${{x-bw/2}}" y="${{top}}" width="${{bw}}" height="${{Math.max(hh,1)}}" rx="2" fill="${{r.color}}"><title>${{r.date}} · ${{r.pattern}} · ${{r.pnl.toFixed(2)}}%</title></rect>`;if(i===0||i===D.trades.length-1||i===7)s+=txt(x,p.t+ch+22,r.date.slice(5),{{anchor:'middle',size:10}})}});s+=txt(p.l,p.t-10,'浮盈（%）',{{fill:'#dbe7e6'}});s+=txt(w-p.r,y(2.5)-7,'+2.5% 达标线',{{anchor:'end',fill:'#f3bf54'}});$('#tradeChart').innerHTML=s}})();</script></body></html>'''
open(OUT,'w',encoding='utf-8').write(html)
print('wrote',OUT)
print(json.dumps({'qualified_patterns':[{k:v for k,v in x.items() if k not in ['rows']} for x in qualified], 'summary':S, 'all':ALL, 'hist':hist},ensure_ascii=False,indent=2))
