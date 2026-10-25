"""Fetch complete dated mainboard universes; retry failed pages, never partial lists."""
import argparse,signal,socket,json
from pathlib import Path
import baostock as bs
import baostock.util.socketutil as transport
import baostock.common.context as context
socket.setdefaulttimeout(8)
signal.signal(signal.SIGALRM,lambda *_: (_ for _ in ()).throw(TimeoutError('metadata request timeout')))
raw_send=transport.send_msg

def send(message):
 for attempt in range(3):
  signal.alarm(12)
  try: result=raw_send(message)
  finally: signal.alarm(0)
  if result is not None: return result
  print('metadata reconnect',attempt+1,flush=True)
  old=getattr(context,'default_socket',None)
  if old: old.close()
  transport.send_msg=raw_send
  signal.alarm(12)
  try: login=bs.login()
  finally: signal.alarm(0);transport.send_msg=send
  if login.error_code!='0': raise RuntimeError(login.error_msg)
 raise ConnectionError('metadata request failed after retries')
parser=argparse.ArgumentParser()
parser.add_argument('--output',required=True)
parser.add_argument('--dates',nargs='+',default=['2026-10-20','2026-10-08','2026-10-09'])
args=parser.parse_args()
transport.send_msg=send
bs.login()
out=Path(args.output)
out.parent.mkdir(parents=True,exist_ok=True)
data=json.loads(out.read_text()) if out.exists() else {}
for day in args.dates:
 if day in data: continue
 r=bs.query_all_stock(day=day);rows=[]
 while r.error_code=='0' and r.next():
  row=dict(zip(r.fields,r.get_row_data()))
  if row['code'].startswith(('sh.60','sz.00')): rows.append(row)
 if r.error_code!='0' or len(rows)<3000: raise RuntimeError(f'{day} incomplete {r.error_code} {len(rows)}')
 data[day]=rows;out.write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n')
 print(json.dumps({'date':day,'mainboard_codes':len(rows)}),flush=True)
# No logout request needed; close the local socket explicitly.
context.default_socket.close()
