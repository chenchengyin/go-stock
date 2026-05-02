assistant: 

assistant: 
reasoning content:
[STEP]🔧 调用工具：QueryStockCodeInfo({"searchWord": "立讯精密"})
assistant: 
reasoning content:
[STEP]✅ QueryStockCodeInfo 返回结果（524字）
assistant: 

assistant: 
reasoning content:
[STEP]🔧 调用工具：get_realtime_data({"symbol": "002475"})
assistant: 
reasoning content:
[STEP]✅ get_realtime_data 返回结果（519字）
assistant: 

assistant: 
reasoning content:
[STEP]🔧 调用工具：get_hist_data({"symbol": "002475", "recent_n": 100, "interval": "day", "adjust": "qfq"})
assistant: 
reasoning content:
[STEP]✅ get_hist_data 返回结果（347字）
assistant: 

assistant: 
reasoning content:
[STEP]🔧 调用工具：GetStockInfo({"stockCode": "sz002475"})
assistant: 
reasoning content:
[STEP]✅ GetStockInfo 返回结果（335字）
assistant: 



assistant: 
reasoning content:
[STEP]🔧 调用工具：GetEastMoneyKLineWithMA({"stockCode": "002475.SZ", "kLineType": "day", "limit": 60, "maPeriods": "5,10,20,60"})
assistant: 
reasoning content:
[STEP]✅ GetEastMoneyKLineWithMA 返回结果（11633字）
assistant: ❌ Agent 调用失败：[NodeRunError] failed to create chat stream completion: HTTP 400 (Code 20015): "messages" in request are illegal.
------------------------
node path: [chat]