package handler

import "net/http"

func MonitorUIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(monitorHTML))
	}
}

var monitorHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Flash Mall - 服务监控</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:#0f1521;color:#e2e8f0;padding:20px}
h1{font-size:20px;margin-bottom:16px;color:#fff}
h2{font-size:16px;margin-bottom:12px;color:#cbd5e1}
.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(260px,1fr));gap:12px;margin-bottom:24px}
.card{background:#1a2332;border-radius:8px;padding:16px;border-left:4px solid #4a5568}
.card.ok{border-left-color:#48bb78}
.card.err{border-left-color:#f56565}
.card h3{font-size:13px;color:#a0aec0;margin-bottom:6px}
.card .val{font-size:26px;font-weight:700}
.card .detail{font-size:12px;color:#718096;margin-top:4px}
table{width:100%;border-collapse:collapse;background:#1a2332;border-radius:8px;overflow:hidden;margin-bottom:24px}
th,td{padding:10px 14px;text-align:left;border-bottom:1px solid #2d3748;font-size:13px}
th{background:#2d3748;color:#a0aec0;font-weight:600}
.ok-text{color:#48bb78}
.err-text{color:#f56565}
.refresh{background:#4a5568;color:#fff;border:none;padding:6px 16px;border-radius:4px;cursor:pointer;font-size:13px}
.refresh:hover{background:#718096}
.header{display:flex;justify-content:space-between;align-items:center;margin-bottom:16px}
.mono{font-family:"Cascadia Code","Fira Code",monospace;font-size:12px;color:#94a3b8}
.section{margin-bottom:28px}
</style>
</head>
<body>
<div class="header">
  <h1>Flash Mall 服务监控</h1>
  <div><span id="ts" style="color:#718096;font-size:12px"></span> <button class="refresh" onclick="loadAll()">刷新</button></div>
</div>

<div class="section">
  <h2>服务状态</h2>
  <div class="grid" id="cards"></div>
  <table><thead><tr><th>依赖项</th><th>状态</th><th>详情</th></tr></thead><tbody id="deps"></tbody></table>
</div>

<div class="section">
  <h2>业务指标 (Prometheus)</h2>
  <div class="grid" id="metrics-cards"></div>
  <table><thead><tr><th>指标名</th><th>标签</th><th>值</th></tr></thead><tbody id="metrics-tbody"></tbody></table>
</div>

<script>
function card(title,val,status,detail){
  return '<div class="card '+status+'"><h3>'+title+'</h3><div class="val">'+val+'</div>'+(detail?'<div class="detail">'+detail+'</div>':'')+'</div>';
}
async function loadHealth(){
  try{
    var r=await fetch('/api/system/health');
    var d=await r.json();
    document.getElementById('ts').textContent='更新于 '+new Date().toLocaleTimeString();
    var cards='';
    cards+=card('整体状态',d.overall?'正常':'异常',d.overall?'ok':'err','');
    cards+=card('运行时间',d.uptime,'ok','');
    cards+=card('Goroutines',d.goroutines,'ok','');
    cards+=card('版本',d.version,'ok','');
    document.getElementById('cards').innerHTML=cards;
    var html='';
    for(var i=0;i<d.dependencies.length;i++){
      var dep=d.dependencies[i];
      html+='<tr><td>'+dep.name+'</td>';
      html+='<td>'+(dep.ok?'<span class="ok-text">正常</span>':'<span class="err-text">异常</span>')+'</td>';
      html+='<td style="color:#718096">'+dep.detail+'</td></tr>';
    }
    document.getElementById('deps').innerHTML=html;
  }catch(e){
    document.getElementById('cards').innerHTML='<div class="card err"><h3>连接失败</h3><div class="val">无法访问</div><div class="detail">'+e.message+'</div></div>';
  }
}

function parsePrometheus(text){
  var metrics=[];
  var lines=text.split('\n');
  for(var i=0;i<lines.length;i++){
    var line=lines[i].trim();
    if(!line||line.charAt(0)==='#')continue;
    var spaceIdx=line.lastIndexOf(' ');
    if(spaceIdx<0)continue;
    var namePart=line.substring(0,spaceIdx);
    var value=line.substring(spaceIdx+1).trim();
    if(isNaN(parseFloat(value)))continue;
    var braceIdx=namePart.indexOf('{');
    var name=namePart;
    var labels='';
    if(braceIdx>=0){
      name=namePart.substring(0,braceIdx);
      labels=namePart.substring(braceIdx+1,namePart.length-1);
    }
    metrics.push({name:name,labels:labels,value:parseFloat(value)});
  }
  return metrics;
}

function formatMetricValue(name,val){
  if(name.indexOf('_total')>=0||name.indexOf('_count')>=0){
    return val.toFixed(0);
  }
  if(name.indexOf('_seconds')>=0){
    return val.toFixed(4)+'s';
  }
  if(name.indexOf('_backlog')>=0){
    return val.toFixed(0);
  }
  return val.toFixed(2);
}

async function loadMetrics(){
  try{
    var r=await fetch('/metrics');
    var text=await r.text();
    var metrics=parsePrometheus(text);

    // Summary cards for key business metrics
    var summary={};
    for(var i=0;i<metrics.length;i++){
      var m=metrics[i];
      if(m.name.indexOf('flashmall_')!==0)continue;
      var key=m.name;
      if(!summary[key])summary[key]=0;
      summary[key]+=m.value;
    }

    var cardHtml='';
    var keyMetrics=[
      'flashmall_order_create_total',
      'flashmall_order_payment_total',
      'flashmall_order_compensation_total',
      'flashmall_catalog_request_total',
      'flashmall_order_delay_queue_backlog'
    ];
    var labels={
      'flashmall_order_create_total':'创建订单',
      'flashmall_order_payment_total':'支付请求',
      'flashmall_order_compensation_total':'补偿次数',
      'flashmall_catalog_request_total':'目录请求',
      'flashmall_order_delay_queue_backlog':'延迟队列积压'
    };
    for(var j=0;j<keyMetrics.length;j++){
      var km=keyMetrics[j];
      if(summary[km]!==undefined){
        cardHtml+=card(labels[km]||km,formatMetricValue(km,summary[km]),'ok','');
      }
    }
    document.getElementById('metrics-cards').innerHTML=cardHtml||'<div class="card"><h3>无业务指标</h3><div class="val">—</div></div>';

    // Full metrics table (only flashmall_* metrics)
    var rows='';
    for(var k=0;k<metrics.length;k++){
      var m=metrics[k];
      if(m.name.indexOf('flashmall_')!==0)continue;
      var displayName=m.name.replace('flashmall_','');
      rows+='<tr><td class="mono">'+displayName+'</td><td class="mono">'+(m.labels||'—')+'</td><td class="mono">'+formatMetricValue(m.name,m.value)+'</td></tr>';
    }
    document.getElementById('metrics-tbody').innerHTML=rows||'<tr><td colspan="3" style="text-align:center;color:#718096">暂无指标数据</td></tr>';
  }catch(e){
    document.getElementById('metrics-cards').innerHTML='<div class="card err"><h3>指标加载失败</h3><div class="val">—</div><div class="detail">'+e.message+'</div></div>';
  }
}

function loadAll(){loadHealth();loadMetrics();}
loadAll();
setInterval(loadAll,10000);
</script>
</body>
</html>`
