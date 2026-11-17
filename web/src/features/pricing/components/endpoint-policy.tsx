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
import { useTranslation } from 'react-i18next'

import { getEndpointTypeLabels, type EndpointTypeOption } from '../constants'
import type { PricingModel } from '../types'

export function EndpointPolicy(props: {
  policy: PricingModel['endpoint_policy']
}) {
  const { t } = useTranslation()
  const policy = props.policy
  if (
    !policy ||
    (policy.mode === 'blacklist' && policy.endpoints.length === 0)
  ) {
    return null
  }

  const labels = getEndpointTypeLabels(t)
  const endpoints =
    policy.endpoints
      .map((name) => labels[name as EndpointTypeOption] ?? name)
      .join(', ') || t('None')
  return (
    <span className='block min-w-0 truncate text-xs' title={endpoints}>
      <span className='text-muted-foreground'>
        {policy.mode === 'whitelist'
          ? t('Only allowed:')
          : t('Not allowed:')}{' '}
      </span>
      {endpoints}
    </span>
  )
}
