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
type Satellite = {
  name: string
  radius: number
  distance: number
  speed: number
}

type SatelliteSystem = {
  inclination: number
  moons: Satellite[]
}

// Distances and sizes are scaled for a readable interactive scene.
export const SATELLITE_SYSTEMS: Record<string, SatelliteSystem> = {
  Earth: {
    inclination: 0.09,
    moons: [{ name: 'Moon', radius: 0.075, distance: 0.68, speed: 0.8 }],
  },
  Mars: {
    inclination: 0.03,
    moons: [
      { name: 'Phobos', radius: 0.035, distance: 0.36, speed: 1.5 },
      { name: 'Deimos', radius: 0.025, distance: 0.55, speed: 0.65 },
    ],
  },
  Jupiter: {
    inclination: 0.03,
    moons: [
      { name: 'Io', radius: 0.12, distance: 1.2, speed: 1.3 },
      { name: 'Europa', radius: 0.1, distance: 1.65, speed: 0.95 },
      { name: 'Ganymede', radius: 0.18, distance: 2.2, speed: 0.7 },
      { name: 'Callisto', radius: 0.16, distance: 2.85, speed: 0.45 },
    ],
  },
  Saturn: {
    inclination: 0.47,
    moons: [
      { name: 'Mimas', radius: 0.035, distance: 1.6, speed: 1.5 },
      { name: 'Enceladus', radius: 0.045, distance: 1.85, speed: 1.2 },
      { name: 'Tethys', radius: 0.06, distance: 2.1, speed: 0.95 },
      { name: 'Dione', radius: 0.065, distance: 2.4, speed: 0.8 },
      { name: 'Rhea', radius: 0.085, distance: 2.75, speed: 0.65 },
      { name: 'Titan', radius: 0.17, distance: 3.15, speed: 0.4 },
      { name: 'Iapetus', radius: 0.085, distance: 3.8, speed: 0.2 },
    ],
  },
  Uranus: {
    inclination: 1.71,
    moons: [
      { name: 'Miranda', radius: 0.035, distance: 0.68, speed: 1.4 },
      { name: 'Ariel', radius: 0.06, distance: 0.88, speed: 1.1 },
      { name: 'Umbriel', radius: 0.06, distance: 1.12, speed: 0.85 },
      { name: 'Titania', radius: 0.08, distance: 1.38, speed: 0.65 },
      { name: 'Oberon', radius: 0.08, distance: 1.68, speed: 0.45 },
    ],
  },
  Neptune: {
    inclination: 0.52,
    moons: [{ name: 'Triton', radius: 0.1, distance: 0.95, speed: -0.75 }],
  },
}
