import { ReactNode } from 'react'
import { INSTALL_COMMAND } from '@/constants'

type DocsPageProps = {
  markdown: string
}

type Block =
  | { type: 'h2'; text: string }
  | { type: 'h3'; text: string }
  | { type: 'p'; text: string }
  | { type: 'ul'; items: string[] }
  | { type: 'ol'; items: string[] }
  | { type: 'code'; code: string }
  | { type: 'blockquote'; text: string }
  | { type: 'table'; header: string[]; rows: string[][] }

function renderInline(text: string): ReactNode[] {
  const parts: ReactNode[] = []
  const regex = /(\[[^\]]+\]\([^\)]+\)|`[^`]+`|\*\*[^*]+\*\*)/g
  let lastIndex = 0

  for (const match of text.matchAll(regex)) {
    const matched = match[0]
    const index = match.index ?? 0

    if (index > lastIndex) {
      parts.push(text.slice(lastIndex, index))
    }

    if (matched.startsWith('`') && matched.endsWith('`')) {
      parts.push(<code key={`${index}-code`}>{matched.slice(1, -1)}</code>)
    } else if (matched.startsWith('**') && matched.endsWith('**')) {
      parts.push(<strong key={`${index}-strong`}>{matched.slice(2, -2)}</strong>)
    } else {
      const linkMatch = matched.match(/^\[([^\]]+)\]\(([^\)]+)\)$/)
      if (linkMatch) {
        parts.push(
          <a key={`${index}-link`} href={linkMatch[2]}>
            {linkMatch[1]}
          </a>,
        )
      }
    }

    lastIndex = index + matched.length
  }

  if (lastIndex < text.length) {
    parts.push(text.slice(lastIndex))
  }

  return parts
}

function parseRow(line: string): string[] {
  const cells = line.split('|').map((cell) => cell.trim())
  const hasLeadingPipe = cells[0] === ''
  const hasTrailingPipe = cells[cells.length - 1] === ''

  const start = hasLeadingPipe ? 1 : 0
  const end = hasTrailingPipe ? cells.length - 1 : cells.length

  return cells.slice(start, end)
}

function parseTable(lines: string[], start: number): [Block | null, number] {
  const first = lines[start]
  const second = lines[start + 1]
  if (!first || !second || !first.includes('|') || !second.includes('|')) {
    return [null, start]
  }

  const isDivider = parseRow(second).every((cell) => /^:?-{3,}:?$/.test(cell))
  if (!isDivider) {
    return [null, start]
  }

  const header = parseRow(first)
  const rows: string[][] = []
  let i = start + 2
  while (i < lines.length && lines[i].includes('|')) {
    rows.push(parseRow(lines[i]))
    i += 1
  }

  return [{ type: 'table', header, rows }, i - 1]
}

function parseMarkdown(markdown: string): Block[] {
  const lines = markdown.split('\n')
  const blocks: Block[] = []

  for (let i = 0; i < lines.length; i += 1) {
    const trimmed = lines[i].trim()

    if (!trimmed) continue

    if (trimmed.startsWith('```')) {
      const codeLines: string[] = []
      i += 1
      while (i < lines.length && !lines[i].trim().startsWith('```')) {
        codeLines.push(lines[i])
        i += 1
      }
      blocks.push({ type: 'code', code: codeLines.join('\n') })
      continue
    }

    const [table, tableEnd] = parseTable(lines, i)
    if (table) {
      blocks.push(table)
      i = tableEnd
      continue
    }

    if (trimmed.startsWith('## ')) {
      blocks.push({ type: 'h2', text: trimmed.replace(/^## /, '') })
      continue
    }

    if (trimmed.startsWith('### ')) {
      blocks.push({ type: 'h3', text: trimmed.replace(/^### /, '') })
      continue
    }

    if (trimmed.startsWith('> ')) {
      blocks.push({ type: 'blockquote', text: trimmed.replace(/^> /, '') })
      continue
    }

    if (/^\d+\. /.test(trimmed)) {
      const items: string[] = []
      while (i < lines.length && /^\d+\. /.test(lines[i].trim())) {
        items.push(lines[i].trim().replace(/^\d+\. /, ''))
        i += 1
      }
      i -= 1
      blocks.push({ type: 'ol', items })
      continue
    }

    if (trimmed.startsWith('- ')) {
      const items: string[] = []
      while (i < lines.length && lines[i].trim().startsWith('- ')) {
        items.push(lines[i].trim().replace(/^- /, ''))
        i += 1
      }
      i -= 1
      blocks.push({ type: 'ul', items })
      continue
    }

    const paragraph: string[] = [trimmed]
    while (i + 1 < lines.length) {
      const next = lines[i + 1].trim()
      if (!next || /^(## |### |> |- |\d+\. |```)/.test(next)) break
      i += 1
      paragraph.push(next)
    }

    blocks.push({ type: 'p', text: paragraph.join(' ') })
  }

  return blocks
}

export default function DocsPage({ markdown }: DocsPageProps) {
  const resolvedMarkdown = markdown.split('{{INSTALL_COMMAND}}').join(INSTALL_COMMAND)
  const blocks = parseMarkdown(resolvedMarkdown)

  return (
    <>
      {blocks.map((block, index) => {
        switch (block.type) {
          case 'h2':
            return <h2 key={index}>{block.text}</h2>
          case 'h3':
            return <h3 key={index}>{block.text}</h3>
          case 'p':
            return <p key={index}>{renderInline(block.text)}</p>
          case 'blockquote':
            return <blockquote key={index}>{renderInline(block.text)}</blockquote>
          case 'code':
            return (
              <pre key={index}>
                <code>{block.code}</code>
              </pre>
            )
          case 'ul':
            return (
              <ul key={index}>
                {block.items.map((item, itemIndex) => (
                  <li key={itemIndex}>{renderInline(item)}</li>
                ))}
              </ul>
            )
          case 'ol':
            return (
              <ol key={index}>
                {block.items.map((item, itemIndex) => (
                  <li key={itemIndex}>{renderInline(item)}</li>
                ))}
              </ol>
            )
          case 'table':
            return (
              <table key={index}>
                <thead>
                  <tr>
                    {block.header.map((cell, cellIndex) => (
                      <th key={cellIndex}>{renderInline(cell)}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {block.rows.map((row, rowIndex) => (
                    <tr key={rowIndex}>
                      {row.map((cell, cellIndex) => (
                        <td key={cellIndex}>{renderInline(cell)}</td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            )
        }
      })}
    </>
  )
}
