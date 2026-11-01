import type { Sale } from '../api'

export function shortSaleId(id: string): string {
  const clean = id.replace(/[^a-zA-Z0-9]/g, '').toUpperCase()
  return clean.slice(0, 8) || id
}
export function receiptNumber(sale: Pick<Sale, 'id' | 'completedAt'>): string {
  const datePart = sale.completedAt ? sale.completedAt.slice(0, 10).replace(/-/g, '') : '00000000'
  return `RCPT-${datePart}-${shortSaleId(sale.id).slice(0, 6)}`
}

export function escapeHtml(value: unknown): string {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function escapeCsvCell(value: unknown): string {
  if (value === null || value === undefined) return ''
  let text = String(value)
  if (/^[=+\-@]/.test(text)) {
    text = `'${text}`
  }
  if (/[",\r\n]/.test(text)) {
    return `"${text.replace(/"/g, '""')}"`
  }
  return text
}

export function downloadCsv(
  filename: string,
  headers: string[],
  rows: Array<Array<string | number | null | undefined>>,
): void {
  const lines = [
    headers.map(escapeCsvCell).join(','),
    ...rows.map(row => row.map(escapeCsvCell).join(',')),
  ]
  const csvContent = `\uFEFF${lines.join('\r\n')}`
  const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

export function openPrintDocument(title: string, bodyHtml: string): void {
  const printWindow = window.open('', '_blank', 'width=900,height=720')
  if (!printWindow) {
    window.print()
    return
  }
  printWindow.document.open()
  printWindow.document.write(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8" />
  <title>${escapeHtml(title)}</title>
  <style>
    * { box-sizing: border-box; }
    body {
      font-family: 'Segoe UI', Arial, sans-serif;
      color: #0f172a;
      background: #ffffff;
      margin: 24px;
      font-size: 13px;
    }
    h1 { margin: 0 0 4px; font-size: 20px; }
    h2 { margin: 16px 0 8px; font-size: 15px; }
    p { margin: 2px 0; color: #475569; }
    .meta {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      border-bottom: 2px solid #e2e8f0;
      padding-bottom: 12px;
      margin-bottom: 16px;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      margin-top: 8px;
    }
    th, td {
      border: 1px solid #cbd5e1;
      padding: 7px 10px;
      text-align: left;
    }
    th {
      background: #f1f5f9;
      font-size: 11px;
      text-transform: uppercase;
      letter-spacing: 0.04em;
    }
    .num { text-align: right; }
    @media print {
      body { margin: 10mm; }
    }
  </style>
</head>
<body>
  ${bodyHtml}
</body>
</html>`)
  printWindow.document.close()
  printWindow.focus()
  setTimeout(() => {
    printWindow.print()
  }, 150)
}
