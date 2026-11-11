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
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { Trans } from 'react-i18next'
import { afterEach, expect, test } from 'vitest'

import { CurrencyText } from '@/components/currency-display'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { mapStatusDataToConfig } from '@/lib/status-query'
import { CURRENCY_TOOLTIP } from '@/lib/vchart'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

const previous = useSystemConfigStore.getState().config
afterEach(() => useSystemConfigStore.getState().setConfig(previous))

test('uses the uploaded icon from status without changing the formatted amount', () => {
  useSystemConfigStore.getState().setConfig(
    mapStatusDataToConfig({
      quota_display_type: 'CUSTOM',
      custom_currency_symbol: 'Credits',
      custom_currency_exchange_rate: 1,
      custom_currency_icon: '/api/appearance/background/coin.png',
    })
  )
  const formatted = formatBillingCurrencyFromUSD(1.23456, { digitsLarge: 5 })
  expect(formatted).toBe('Credits 1.23456')
  const view = render(<CurrencyText>{formatted}</CurrencyText>)
  expect(screen.getByRole('img', { name: 'Credits' })).toHaveAttribute(
    'src',
    '/api/appearance/background/coin.png'
  )
  expect(view.container).toHaveTextContent('1.23456')
  expect(screen.getByRole('img').parentElement).toHaveClass(
    'inline-flex',
    'whitespace-nowrap'
  )
  fireEvent.error(screen.getByRole('img'))
  expect(screen.queryByRole('img')).not.toBeInTheDocument()
  expect(view.container).toHaveTextContent(formatted)
  act(() =>
    useSystemConfigStore.getState().setConfig({
      currency: {
        ...useSystemConfigStore.getState().config.currency,
        customCurrencyIcon: '/api/appearance/background/replacement.png',
      },
    })
  )
  expect(screen.getByRole('img')).toHaveAttribute(
    'src',
    '/api/appearance/background/replacement.png'
  )
})

test('replaces monetary chart values without changing model names or precision', () => {
  useSystemConfigStore.getState().setConfig({
    currency: {
      ...DEFAULT_CURRENCY_CONFIG,
      quotaDisplayType: 'CUSTOM',
      customCurrencySymbol: 'Credits',
      customCurrencyIcon: '/coin.png',
    },
  })
  const tooltip = document.createElement('div')
  tooltip.innerHTML =
    '<div data-col="key"><div>Credits model</div></div><div data-col="value"><div>Credits0.9245</div><div>42</div></div>'
  CURRENCY_TOOLTIP.updateElement(tooltip)
  const image = within(tooltip).getByRole('img')
  expect(image).toHaveAttribute('src', '/coin.png')
  expect(image.parentElement).toHaveTextContent('0.9245')
  expect(tooltip.querySelector('[data-col="key"]')).toHaveTextContent(
    'Credits model'
  )
  expect(tooltip.querySelectorAll('img')).toHaveLength(1)
  fireEvent.error(image)
  expect(tooltip.querySelector('[data-col="value"]')).toHaveTextContent(
    'Credits0.9245'
  )
})

test('keeps text symbols when no image is configured and for other currencies', () => {
  useSystemConfigStore.getState().setConfig({
    currency: {
      ...DEFAULT_CURRENCY_CONFIG,
      quotaDisplayType: 'CUSTOM',
      customCurrencySymbol: 'Credits',
    },
  })
  const view = render(<CurrencyText>Credits 2.5</CurrencyText>)
  expect(screen.queryByRole('img')).not.toBeInTheDocument()
  expect(view.container).toHaveTextContent('Credits 2.5')
  act(() =>
    useSystemConfigStore.getState().setConfig({
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        customCurrencyIcon: '/coin.png',
      },
    })
  )
  view.rerender(<CurrencyText>$2.5</CurrencyText>)
  expect(screen.queryByRole('img')).not.toBeInTheDocument()
  expect(view.container).toHaveTextContent('$2.5')
})

test('uses the same image in column labels and translated monetary hints', () => {
  useSystemConfigStore.getState().setConfig({
    currency: {
      ...DEFAULT_CURRENCY_CONFIG,
      quotaDisplayType: 'CUSTOM',
      customCurrencySymbol: 'Credits',
      customCurrencyIcon: '/coin.png',
    },
  })
  const view = render(
    <>
      <CurrencyText label>Available Balance (Credits)</CurrencyText>
      <Trans
        i18nKey='Leave empty to use the site default (<amount></amount>)'
        components={{
          amount: (
            <span>
              <CurrencyText>Credits 0.002</CurrencyText>
            </span>
          ),
        }}
      />
    </>
  )
  expect(screen.getAllByRole('img', { name: 'Credits' })).toHaveLength(2)
  expect(view.container).toHaveTextContent('0.002')
  expect(view.container).not.toHaveTextContent('(Credits)')
})
