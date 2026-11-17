/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useLayoutEffect, useRef, type ReactNode } from 'react'

import { cn } from '@/lib/utils'

export function TableScrollArea(props: {
  children: ReactNode
  className?: string
}) {
  const viewportRef = useRef<HTMLDivElement>(null)

  useLayoutEffect(() => {
    const viewport = viewportRef.current
    const frame = viewport?.closest<HTMLElement>(
      '[data-slot="data-table-frame"]'
    )
    const table = viewport?.querySelector('table')
    const header = table?.tHead
    if (!viewport || !frame || !table || !header) return

    const cells = [...header.querySelectorAll('th')]
    const measure = () => {
      const frameStyle = getComputedStyle(frame)
      const radius = Math.max(
        0,
        Number.parseFloat(frameStyle.borderTopLeftRadius) -
          Number.parseFloat(frameStyle.borderLeftWidth)
      )
      let leftWidth = 0
      let rightWidth = 0
      for (const cell of cells) {
        const style = getComputedStyle(cell)
        if (cell.classList.contains('sticky')) {
          if (style.left !== 'auto') {
            leftWidth = Math.max(
              leftWidth,
              cell.offsetWidth + Number.parseFloat(style.left)
            )
          }
          if (style.right !== 'auto') {
            rightWidth = Math.max(
              rightWidth,
              cell.offsetWidth + Number.parseFloat(style.right)
            )
          }
        }
        cell.style.setProperty(
          '--table-header-top',
          `${(cell.parentElement?.offsetTop ?? header.offsetTop) - header.offsetTop}px`
        )
      }

      viewport.style.setProperty(
        '--table-viewport-width',
        `${viewport.clientWidth}px`
      )
      viewport.style.setProperty(
        '--table-viewport-height',
        `${viewport.clientHeight}px`
      )
      viewport.style.setProperty(
        '--table-scrollbar-width',
        `${viewport.offsetWidth - viewport.clientWidth}px`
      )
      viewport.style.setProperty(
        '--table-scrollbar-height',
        `${viewport.offsetHeight - viewport.clientHeight}px`
      )
      viewport.style.setProperty('--table-inner-radius', `${radius}px`)
      viewport.style.setProperty(
        '--table-header-height',
        `${header.offsetHeight}px`
      )
      viewport.style.setProperty('--table-left-width', `${leftWidth}px`)
      viewport.style.setProperty('--table-right-width', `${rightWidth}px`)
      viewport.toggleAttribute('data-pinned-left', leftWidth > 0)
      viewport.toggleAttribute('data-pinned-right', rightWidth > 0)
    }

    measure()
    const observer = new ResizeObserver(measure)
    for (const element of [viewport, frame, table, header, ...cells]) {
      observer.observe(element)
    }
    return () => observer.disconnect()
  }, [props.children])

  return (
    <div
      ref={viewportRef}
      className={cn(
        'table-scroll-area relative w-full overflow-auto',
        props.className
      )}
    >
      <div className='relative min-h-full w-max min-w-full'>
        <div
          aria-hidden='true'
          className='table-scroll-glass table-scroll-header-glass'
        />
        <div
          aria-hidden='true'
          className='table-scroll-glass table-scroll-left-glass'
        />
        <div
          aria-hidden='true'
          className='table-scroll-glass table-scroll-right-glass'
        />
        <div
          aria-hidden='true'
          className='table-scroll-glass table-scroll-left-corner-glass'
        />
        <div
          aria-hidden='true'
          className='table-scroll-glass table-scroll-right-corner-glass'
        />
        {props.children}
      </div>
    </div>
  )
}
