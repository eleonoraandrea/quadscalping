package report

// reportTpl è il template HTML del report (self-contained).
// Palette e specifiche: linea 2px, griglia hairline, tick puliti, testo mai
// colorato come la serie, tabella con tabular-nums, dark mode selezionato.
const reportTpl = `<!DOCTYPE html>
<html lang="it">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Title}}</title>
<style>
  :root { color-scheme: light; }
  .viz-root {
    color-scheme: light;
    --page:            #f9f9f7;
    --surface:         #fcfcfb;
    --text-primary:    #0b0b0b;
    --text-secondary:  #52514e;
    --muted:           #898781;
    --grid:            #e1e0d9;
    --axis:            #c3c2b7;
    --series-1:        #2a78d6;
    --neg:             #e34948;
    --pos-text:        #006300;
    --neg-text:        #d03b3b;
    --border:          rgba(11,11,11,0.10);
  }
  @media (prefers-color-scheme: dark) {
    :root:where(:not([data-theme="light"])) .viz-root {
      color-scheme: dark;
      --page:            #0d0d0d;
      --surface:         #1a1a19;
      --text-primary:    #ffffff;
      --text-secondary:  #c3c2b7;
      --muted:           #898781;
      --grid:            #2c2c2a;
      --axis:            #383835;
      --series-1:        #3987e5;
      --neg:             #e66767;
      --pos-text:        #0ca30c;
      --neg-text:        #e66767;
      --border:          rgba(255,255,255,0.10);
    }
  }
  :root[data-theme="dark"] .viz-root {
    color-scheme: dark;
    --page:            #0d0d0d;
    --surface:         #1a1a19;
    --text-primary:    #ffffff;
    --text-secondary:  #c3c2b7;
    --muted:           #898781;
    --grid:            #2c2c2a;
    --axis:            #383835;
    --series-1:        #3987e5;
    --neg:             #e66767;
    --pos-text:        #0ca30c;
    --neg-text:        #e66767;
    --border:          rgba(255,255,255,0.10);
  }
  :root[data-theme="light"] .viz-root { color-scheme: light; }

  * { box-sizing: border-box; }
  body {
    margin: 0; background: var(--page); color: var(--text-primary);
    font-family: system-ui, -apple-system, "Segoe UI", sans-serif;
    font-size: 15px; line-height: 1.45;
  }
  .wrap { max-width: 980px; margin: 0 auto; padding: 28px 20px 60px; }
  header.page h1 { font-size: 24px; margin: 0 0 4px; font-weight: 650; }
  header.page .meta { color: var(--text-secondary); font-size: 13px; }
  .theme-toggle {
    float: right; background: var(--surface); color: var(--text-secondary);
    border: 1px solid var(--border); border-radius: 8px; padding: 6px 12px;
    font: inherit; cursor: pointer;
  }
  section.sym { margin-top: 34px; }
  section.sym > h2 { font-size: 18px; margin: 0 0 2px; }
  section.sym > .meta { color: var(--text-secondary); font-size: 13px; margin-bottom: 14px; }

  .tiles { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 12px; }
  .tile {
    background: var(--surface); border: 1px solid var(--border);
    border-radius: 12px; padding: 14px 16px; min-width: 0;
  }
  .tile .lbl { color: var(--text-secondary); font-size: 12.5px; }
  .tile .val { font-size: 21px; font-weight: 600; margin-top: 2px;
    overflow-wrap: anywhere; }
  .tile.hero { grid-column: span 2; }
  .tile.hero .val { font-size: clamp(26px, 4.5vw, 40px); }
  .pos { color: var(--pos-text); }
  .neg { color: var(--neg-text); }

  .card {
    background: var(--surface); border: 1px solid var(--border);
    border-radius: 12px; padding: 16px; margin-top: 14px;
  }
  .card h3 { margin: 0 0 10px; font-size: 14px; font-weight: 600;
    color: var(--text-secondary); text-transform: uppercase; letter-spacing: 0.4px; }
  svg.chart { width: 100%; height: auto; display: block; }
  svg.chart .grid { stroke: var(--grid); stroke-width: 1; }
  svg.chart .axis { stroke: var(--axis); stroke-width: 1; }
  svg.chart .tick { fill: var(--muted); font-size: 11px; font-variant-numeric: tabular-nums; }
  svg.chart .cross { stroke: var(--muted); stroke-width: 1; }

  .tip {
    position: fixed; display: none; pointer-events: none; z-index: 10;
    background: var(--surface); border: 1px solid var(--border); border-radius: 8px;
    padding: 6px 10px; font-size: 12.5px; box-shadow: 0 4px 14px rgba(0,0,0,0.12);
  }
  .tip .t { color: var(--text-secondary); }
  .tip .v { font-weight: 600; font-variant-numeric: tabular-nums; }

  table.data { border-collapse: collapse; width: 100%; font-size: 13px; }
  table.data th {
    text-align: left; color: var(--text-secondary); font-weight: 600;
    padding: 6px 8px; border-bottom: 1px solid var(--axis); white-space: nowrap;
  }
  table.data td {
    padding: 5px 8px; border-bottom: 1px solid var(--grid);
    font-variant-numeric: tabular-nums; white-space: nowrap;
  }
  .scrollbox { max-height: 420px; overflow-y: auto; }
  footer.page { margin-top: 40px; color: var(--muted); font-size: 12.5px; }
  .params { color: var(--text-secondary); font-size: 12.5px; }
  .params code { font-variant-numeric: tabular-nums; }
</style>
</head>
<body>
<div class="viz-root wrap">
  <button class="theme-toggle" id="themeToggle" type="button">Tema</button>
  <header class="page">
    <h1>{{.Title}}</h1>
    <div class="meta">Generato {{.GeneratedAt}} · strategia HPS Quad Super Signal</div>
  </header>

  {{range .Symbols}}
  <section class="sym">
    <h2>{{.Symbol}} · {{.Timeframe}}</h2>
    <div class="meta">Periodo {{.Period}} · equity finale {{.FinalEq}} · rischio/trade {{printf "%.2f" .Cfg.RiskPct}}% · commissioni {{printf "%.4f" .Cfg.Commission}}/lato · slippage {{printf "%.4f" .Cfg.Slippage}}/lato</div>

    <div class="tiles">
      <div class="tile hero">
        <div class="lbl">PnL netto</div>
        <div class="val {{pnlcls .M.NetPnL}}">{{.NetPnL}}</div>
      </div>
      <div class="tile"><div class="lbl">Trade</div><div class="val">{{.M.TotalTrades}}</div></div>
      <div class="tile"><div class="lbl">Win rate</div><div class="val">{{printf "%.1f" .M.WinRate}}%</div></div>
      <div class="tile"><div class="lbl">Profit factor</div><div class="val">{{printf "%.2f" .M.ProfitFactor}}</div></div>
      <div class="tile"><div class="lbl">Max drawdown</div>
        <div class="val neg">{{printf "%.1f" .M.MaxDrawdownPct}}%</div>
        <div class="lbl">{{money .M.MaxDrawdown}}</div></div>
      <div class="tile"><div class="lbl">Expectancy</div><div class="val {{pnlcls .M.Expectancy}}">{{money .M.Expectancy}}</div></div>
      <div class="tile"><div class="lbl">SQN</div><div class="val">{{printf "%.2f" .M.SQN}}</div></div>
      <div class="tile"><div class="lbl">Sharpe</div><div class="val">{{printf "%.2f" .M.Sharpe}}</div></div>
      <div class="tile"><div class="lbl">Commissioni</div><div class="val">{{money .M.FeesPaid}}</div></div>
      <div class="tile"><div class="lbl">Esposizione</div><div class="val">{{printf "%.1f" (mul .M.Exposure 100)}}%</div></div>
    </div>

    <div class="card">
      <h3>Equity</h3>
      {{.EquitySVG}}
    </div>
    <div class="card">
      <h3>Drawdown</h3>
      {{.DDSVG}}
    </div>
    {{if .MonthSVG}}
    <div class="card">
      <h3>PnL mensile</h3>
      {{.MonthSVG}}
    </div>
    {{end}}

    <div class="card">
      <h3>Trade ({{.M.TotalTrades}})</h3>
      <div class="scrollbox">
      <table class="data">
        <thead><tr>
          <th>#</th><th>Entry</th><th>Exit</th><th>Motivo</th>
          <th>Prezzo in</th><th>Prezzo out</th><th>Size</th>
          <th>PnL</th><th>R</th><th>Parz.</th>
        </tr></thead>
        <tbody>
        {{range .TradeRows}}
          <tr>
            <td>{{.Idx}}</td><td>{{.Entry}}</td><td>{{.Exit}}</td><td>{{.Motive}}</td>
            <td>{{.EntryPx}}</td><td>{{.ExitPx}}</td><td>{{.Size}}</td>
            <td class="{{.PnlClass}}">{{.PnL}}</td><td>{{.R}}</td><td>{{.Partial}}</td>
          </tr>
        {{end}}
        </tbody>
      </table>
      </div>
    </div>

    <div class="params">
      Parametri HPS: stop ATR <code>{{printf "%.1f" .Cfg.Params.StopATR}}×</code> ·
      TP1 <code>{{printf "%.1f" .Cfg.Params.TP1R}}R</code> ·
      parziale <code>{{printf "%.0f" (mul .Cfg.Params.PartialPct 100)}}%</code> ·
      breakeven dopo TP1 <code>{{if .Cfg.Params.BreakevenOnTP1}}sì{{else}}no{{end}}</code> ·
      slow exit &gt; <code>{{printf "%.0f" .Cfg.Params.ExitSlow}}</code> ·
      cooldown <code>{{.Cfg.CooldownBars}}</code> barre ·
      leva max <code>{{printf "%.1f" .Cfg.MaxLeverage}}×</code>
    </div>
  </section>
  {{end}}

  <footer class="page">
    Report generato da quadscalping. Il trading comporta rischi significativi;
    i risultati passati non garantiscono risultati futuri. Commissioni e slippage
    sono modellati deterministicamente; i fill reali possono differire.
  </footer>
</div>

<div class="tip" id="tip"></div>

<script>
(function () {
  var toggle = document.getElementById('themeToggle');
  var root = document.documentElement;
  toggle.addEventListener('click', function () {
    var cur = root.getAttribute('data-theme');
    var dark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    var next = cur ? null : (dark ? 'light' : 'dark');
    if (next) root.setAttribute('data-theme', next); else root.removeAttribute('data-theme');
  });

  var tip = document.getElementById('tip');

  function fmtDate(ms) {
    var d = new Date(ms);
    var p = function (n) { return (n < 10 ? '0' : '') + n; };
    return p(d.getUTCDate()) + '-' + p(d.getUTCMonth() + 1) + '-' + d.getUTCFullYear() +
      ' ' + p(d.getUTCHours()) + ':' + p(d.getUTCMinutes());
  }
  function fmtNum(v) {
    return v.toLocaleString('it-IT', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  }

  // hover per i grafici a linea: crosshair + tooltip sul punto più vicino
  document.querySelectorAll('svg[data-chart="line"]').forEach(function (svg) {
    if (!svg.dataset.points) return;
    var pts = JSON.parse(svg.dataset.points);
    if (!Array.isArray(pts) || pts.length < 2) return;
    var cross = svg.querySelector('.cross');
    var dot = svg.querySelector('.hdot');
    var hit = svg.querySelector('.hitzone');
    if (!cross || !dot || !hit) return;

    hit.addEventListener('mousemove', function (ev) {
      var rect = svg.getBoundingClientRect();
      var fx = (ev.clientX - rect.left) / rect.width;
      var inner = svg.viewBox.baseVal;
      var ml = 56, mr = 16;
      var pw = inner.width - ml - mr;
      var x = ml + fx * inner.width;
      var idx = Math.round((x - ml) / pw * (pts.length - 1));
      idx = Math.max(0, Math.min(pts.length - 1, idx));
      var px = ml + pw * idx / (pts.length - 1);
      cross.setAttribute('x1', px); cross.setAttribute('x2', px);
      cross.style.display = '';
      // y del punto: ricalcolata dal valore con la scala del grafico
      var vals = pts.map(function (p) { return p.v; });
      var mn = Math.min.apply(null, vals), mx = Math.max.apply(null, vals);
      var mt = 12, mb = 28, ph = inner.height - mt - mb;
      var py = mt + ph * (1 - (pts[idx].v - mn) / (mx - mn || 1));
      dot.setAttribute('cx', px); dot.setAttribute('cy', py);
      dot.style.display = '';
      tip.innerHTML = '<div class="t">' + fmtDate(pts[idx].t) + '</div>' +
        '<div class="v">' + fmtNum(pts[idx].v) + '</div>';
      tip.style.display = 'block';
      tip.style.left = Math.min(ev.clientX + 14, window.innerWidth - 160) + 'px';
      tip.style.top = (ev.clientY - 40) + 'px';
    });
    hit.addEventListener('mouseleave', function () {
      cross.style.display = 'none'; dot.style.display = 'none'; tip.style.display = 'none';
    });
  });

  // hover barre mensili
  document.querySelectorAll('svg.chart rect.bar').forEach(function (bar) {
    bar.addEventListener('mousemove', function (ev) {
      tip.innerHTML = '<div class="t">' + bar.dataset.lbl + '</div>' +
        '<div class="v">' + bar.dataset.val + '</div>';
      tip.style.display = 'block';
      tip.style.left = Math.min(ev.clientX + 14, window.innerWidth - 160) + 'px';
      tip.style.top = (ev.clientY - 40) + 'px';
    });
    bar.addEventListener('mouseleave', function () { tip.style.display = 'none'; });
  });
})();
</script>
</body>
</html>
`
