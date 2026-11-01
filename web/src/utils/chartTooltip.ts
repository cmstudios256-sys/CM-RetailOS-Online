interface ChartTooltipContext {
  chart: {
    id: string
    canvas: HTMLCanvasElement
  }
  tooltip: {
    opacity: number
    title?: string[]
    body?: Array<{ lines: string[] }>
    labelColors: Array<{ backgroundColor: unknown; borderColor: unknown }>
    caretX: number
    caretY: number
  }
}
export const externalChartTooltipHandler = ({ chart, tooltip }: ChartTooltipContext) => {
  const tooltipSelector = `[data-chart-tooltip="${chart.id}"]`
  let tooltipEl = document.querySelector<HTMLDivElement>(tooltipSelector)

  if (!tooltipEl) {
    tooltipEl = document.createElement('div')
    tooltipEl.classList.add('glass-tooltip')
    tooltipEl.dataset.chartTooltip = String(chart.id)

    Object.assign(tooltipEl.style, {
      background: 'var(--bg-surface, #0f172a)',
      backdropFilter: 'blur(16px)',
      WebkitBackdropFilter: 'blur(16px)',
      borderRadius: '8px',
      border: '1px solid var(--border-color, #334155)',
      color: 'var(--text-primary, #f8fafc)',
      opacity: '0',
      pointerEvents: 'none',
      position: 'fixed',
      inset: 'auto',
      margin: '0',
      transform: 'translate(-50%, -100%)',
      transition: 'opacity .2s cubic-bezier(0.4, 0, 0.2, 1)',
      boxShadow: '0 10px 30px -5px rgba(0, 0, 0, 0.45)',
      padding: '10px 14px',
      zIndex: '10000',
      maxWidth: '90vw',
      whiteSpace: 'normal',
    })

    const table = document.createElement('table')
    table.style.margin = '0'
    tooltipEl.appendChild(table)
    document.body.appendChild(tooltipEl)
  }

  if (tooltip.opacity === 0) {
    tooltipEl.style.opacity = '0'
    return
  }

  if (tooltip.body) {
    const titleLines = tooltip.title || []
    const bodyLines = tooltip.body.map(body => body.lines)

    let innerHtml = '<thead>'
    titleLines.forEach(title => {
      innerHtml += '<tr><th style="text-align: left; padding-bottom: 4px; border-bottom: 1px solid var(--border-color, #334155); font-size: 13px; font-weight: 700;">' + title + '</th></tr>'
    })
    innerHtml += '</thead><tbody>'

    bodyLines.forEach((body, index) => {
      const colors = tooltip.labelColors[index] || { backgroundColor: '#0ea5e9', borderColor: '#0ea5e9' }
      let style = 'background:' + colors.backgroundColor
      style += '; border-color:' + colors.borderColor
      style += '; border-width: 2px'
      style += '; width: 10px; height: 10px; display: inline-block; flex: 0 0 10px; margin-right: 8px; border-radius: 2px;'
      const span = '<span style="' + style + '"></span>'
      innerHtml += '<tr><td style="padding-top: 4px; font-size: 12px; font-weight: 500; display: flex; align-items: center;">' + span + body + '</td></tr>'
    })
    innerHtml += '</tbody>'

    const tableRoot = tooltipEl.querySelector('table')
    if (tableRoot) tableRoot.innerHTML = innerHtml
  }

  const canvasRect = chart.canvas.getBoundingClientRect()
  const anchorX = canvasRect.left + tooltip.caretX
  const anchorY = canvasRect.top + tooltip.caretY
  const tooltipRect = tooltipEl.getBoundingClientRect()
  const padding = 8
  const minLeft = tooltipRect.width / 2 + padding
  const maxLeft = window.innerWidth - tooltipRect.width / 2 - padding

  tooltipEl.style.left = `${Math.max(minLeft, Math.min(anchorX, maxLeft))}px`
  tooltipEl.style.top = `${anchorY}px`
  if (anchorY - tooltipRect.height - 10 >= padding) {
    tooltipEl.style.transform = 'translate(-50%, -100%)'
  } else {
    tooltipEl.style.transform = 'translate(-50%, 0)'
  }
  tooltipEl.style.opacity = '1'
}
