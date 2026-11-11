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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'

import { ThemeProvider } from '@/context/theme-provider'

import { SolarHero } from '../components/solar-hero'

afterEach(cleanup)

it('keeps navigation and planet selection available when the device cannot create a 3D view', async () => {
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null)
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const root = createRootRoute({
    component: () => (
      <ThemeProvider>
        <SolarHero isAuthenticated={false} />
      </ThemeProvider>
    ),
  })
  const router = createRouter({
    routeTree: root,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  expect(
    await screen.findByText('3D view is unavailable on this device.')
  ).toBeVisible()
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: 'Choose a planet' }))
  await user.click(screen.getByRole('button', { name: 'Earth' }))
  expect(
    screen.getByRole('button', { name: 'Choose a planet' })
  ).toHaveTextContent('Earth')
  await user.click(screen.getByRole('button', { name: 'Reset view' }))
  expect(
    screen.getByRole('button', { name: 'Choose a planet' })
  ).toHaveTextContent('Solar system')
  await user.click(screen.getByRole('button', { name: 'Pause animation' }))
  expect(
    screen.getByRole('button', { name: 'Resume animation' })
  ).toHaveAttribute('aria-pressed', 'true')
  expect(
    screen.getByRole('button', { name: /Open playground/ })
  ).toHaveAttribute('href', '/sign-in')
  client.clear()
})
