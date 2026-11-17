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
import { getCoreRowModel, useReactTable } from '@tanstack/react-table'
import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'

import { Button } from '@/components/ui/button'

import { DataTableView } from '../data-table-view'

const data = [
  { model: 'GPT', calls: 12 },
  { model: 'Claude', calls: 8 },
]
const columns = [
  { accessorKey: 'model', header: 'Model' },
  { accessorKey: 'calls', header: 'Calls' },
  { id: 'actions', header: 'Actions', cell: () => <Button>Inspect</Button> },
]

function Fixture(props: { splitHeader?: boolean }) {
  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
  })
  return (
    <>
      <Button onClick={() => table.getColumn('calls')?.toggleVisibility()}>
        Toggle calls
      </Button>
      <DataTableView
        table={table}
        splitHeader={props.splitHeader}
        pinnedColumns={[
          { columnId: 'model', side: 'left' },
          { columnId: 'actions', side: 'right' },
        ]}
      />
    </>
  )
}

it.each([true, false])(
  'retains one accessible table and all row content during native scroll (fixed height: %s)',
  (splitHeader) => {
    render(<Fixture splitHeader={splitHeader} />)
    const table = screen.getByRole<HTMLTableElement>('table')
    const viewport = table.closest('.table-scroll-area')
    if (!viewport) throw new Error('Missing table scroll viewport')
    fireEvent.scroll(viewport, { target: { scrollTop: 80, scrollLeft: 100 } })
    expect(screen.getAllByRole('table')).toHaveLength(1)
    expect(within(table).getAllByRole('columnheader')).toHaveLength(3)
    expect(within(table).getByText('GPT')).toBeVisible()
    expect(within(table).getByText('Claude')).toBeVisible()
    expect(
      within(table).getAllByRole('button', { name: 'Inspect' })
    ).toHaveLength(2)
    expect(table.tBodies[0].style.clipPath).toBe('')
  }
)

it('keeps headers and pinned actions available after changing visible columns', async () => {
  render(<Fixture splitHeader />)
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: 'Toggle calls' }))
  expect(
    screen.queryByRole('columnheader', { name: 'Calls' })
  ).not.toBeInTheDocument()
  expect(screen.getByRole('columnheader', { name: 'Model' })).toBeVisible()
  expect(screen.getAllByRole('button', { name: 'Inspect' })).toHaveLength(2)
  await user.click(screen.getByRole('button', { name: 'Toggle calls' }))
  expect(screen.getByRole('columnheader', { name: 'Calls' })).toBeVisible()
  expect(screen.getByRole('cell', { name: '12' })).toBeVisible()
})

it('keeps the fixed header at the top when layout is measured after scrolling', async () => {
  const offsetTop = vi.spyOn(HTMLElement.prototype, 'offsetTop', 'get')
  offsetTop.mockImplementation(function (this: HTMLElement) {
    return this.tagName === 'TH' ? 300 : 0
  })
  try {
    render(<Fixture splitHeader />)
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Toggle calls' }))
    const header = screen.getByRole<HTMLElement>('columnheader', {
      name: 'Model',
    })
    expect(header.style.getPropertyValue('--table-header-top')).toBe('0px')
  } finally {
    offsetTop.mockRestore()
  }
})
