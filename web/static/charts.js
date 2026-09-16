/* Browser code is limited to rendering stored observations. No provider calls. */
(() => {
  const node = document.getElementById('stock-data');
  if (!node || !window.echarts) return;
  const data = JSON.parse(node.dataset.chart);
  const financials = (data.financials || []).slice().reverse();
  const history = (data.history || []).slice().reverse();
  const charts = [];
  function draw(id, options) {
    const element = document.getElementById(id);
    if (!element) return;
    const chart = echarts.init(element);
    chart.setOption({animation:false, aria:{enabled:true}, color:['#4776b4','#75a68a','#ac799b'], grid:{left:70,right:25,top:45,bottom:55}, tooltip:{trigger:'axis',renderMode:'richText'}, ...options});
    charts.push(chart);
  }
  for (const [id,key] of [['revenue-chart','Revenue'],['eps-chart','DilutedEPS']]) {
    const currency = [...new Set(financials.map(f => f.Currency).filter(Boolean))];
    draw(id,{title:{text:financials.length ? '' : 'N/A — no quarterly observations',textStyle:{fontSize:13,fontWeight:'normal',color:'#647184'}},xAxis:{type:'category',data:financials.map(f=>f.PeriodEnd.slice(0,10)),axisLabel:{rotate:30}},yAxis:{type:'value',name:currency.length===1?currency[0]:'Reported currency',axisLabel:{formatter:v=>Math.abs(v)>=1e9?(v/1e9).toFixed(1)+'B':v}},series:[{type:'bar',name:key,data:financials.map(f=>f[key]),barMaxWidth:30}]});
  }
  draw('reaction-chart',{legend:{data:['Premarket','Event day','1W']},xAxis:{type:'category',data:history.map(h=>h.Event.ReportDate.slice(0,10))},yAxis:{type:'value',axisLabel:{formatter:'{value}%'}},tooltip:{trigger:'axis',renderMode:'richText',formatter:items=>{
    if(!items.length)return '';
    const h=history[items[0].dataIndex],e=h.Event;
    const fmt=v=>v==null?'N/A':v.toFixed(2)+'%';
    return [e.ReportDate.slice(0,10),'Quarter: '+(e.FiscalYear&&e.FiscalQuarter?e.FiscalYear+' Q'+e.FiscalQuarter:'N/A'),'EPS surprise: '+fmt(e.EPSSurprisePct),'Revenue surprise: '+fmt(e.RevenueSurprisePct),...items.map(p=>p.seriesName+': '+fmt(p.value))].join('\n');
  }},series:[['Premarket',0],['Event day',1],['1W',4]].map(([name,index])=>({name,type:'line',connectNulls:false,data:history.map(h=>h.Reaction.Returns[index]==null?null:h.Reaction.Returns[index]*100)}))});
  window.addEventListener('resize',()=>charts.forEach(c=>c.resize()));
})();
