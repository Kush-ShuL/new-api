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
import * as THREE from 'three'
import { OrbitControls } from 'three/addons/controls/OrbitControls.js'

import { SATELLITE_SYSTEMS } from './solar-moons'

export const PLANETS = [
  {
    name: 'Mercury',
    texture: 'mercury',
    radius: 0.14,
    distance: 2.1,
    phase: 2.4,
    speed: 0.25,
  },
  {
    name: 'Venus',
    texture: 'venus',
    radius: 0.24,
    distance: 3.2,
    phase: 4.8,
    speed: 0.18,
  },
  {
    name: 'Earth',
    texture: 'earth',
    radius: 0.27,
    distance: 4.3,
    phase: 1.8,
    speed: 0.14,
  },
  {
    name: 'Mars',
    texture: 'mars',
    radius: 0.19,
    distance: 5.5,
    phase: 3.6,
    speed: 0.11,
  },
  {
    name: 'Jupiter',
    texture: 'jupiter',
    radius: 0.78,
    distance: 7.8,
    phase: 0.35,
    speed: 0.065,
  },
  {
    name: 'Saturn',
    texture: 'saturn',
    radius: 0.65,
    distance: 10.3,
    phase: 2.8,
    speed: 0.045,
  },
  {
    name: 'Uranus',
    texture: 'uranus',
    radius: 0.44,
    distance: 12.5,
    phase: 5.65,
    speed: 0.032,
  },
  {
    name: 'Neptune',
    texture: 'neptune',
    radius: 0.42,
    distance: 14.8,
    phase: 1.1,
    speed: 0.025,
  },
] as const

const STAR_VERTEX = `
attribute float a_size;
attribute vec3 a_color;
uniform vec2 u_pointer;
uniform float u_strength;
uniform float u_pixelRatio;
varying float v_light;
varying float v_spike;
varying vec3 v_color;
void main() {
  vec4 projected = projectionMatrix * modelViewMatrix * vec4(position, 1.);
  vec2 delta = projected.xy / projected.w - u_pointer;
  float influence = exp(-dot(delta, delta) * 42.) * u_strength;
  projected.xy += normalize(delta + .0001) * influence * .065 * projected.w;
  gl_Position = projected;
  v_spike = step(3.27, a_size);
  gl_PointSize = mix(a_size, 18., v_spike) * u_pixelRatio + influence * 1.5;
  v_light = .45 + a_size * .2;
  v_color = a_color;
}`
const STAR_FRAGMENT = `
varying float v_light;
varying float v_spike;
varying vec3 v_color;
void main() {
  vec2 point = gl_PointCoord - .5;
  float core = 1. - smoothstep(.015, mix(.5, .12, v_spike), length(point));
  float rays = exp(-abs(point.x) * 110.) * exp(-abs(point.y) * 8.)
             + exp(-abs(point.y) * 110.) * exp(-abs(point.x) * 8.);
  gl_FragColor = vec4(v_color, (core + rays * v_spike * .3) * v_light);
  #include <tonemapping_fragment>
  #include <colorspace_fragment>
}`

const ATMOSPHERE_VERTEX = `
varying vec3 v_worldPosition;
varying vec3 v_worldNormal;
void main() {
  vec4 world = modelMatrix * vec4(position, 1.);
  v_worldPosition = world.xyz;
  v_worldNormal = normalize(mat3(modelMatrix) * normal);
  gl_Position = projectionMatrix * viewMatrix * world;
}`
const ATMOSPHERE_FRAGMENT = `
uniform vec3 u_color;
varying vec3 v_worldPosition;
varying vec3 v_worldNormal;
void main() {
  vec3 normal = normalize(v_worldNormal);
  vec3 viewDirection = normalize(cameraPosition - v_worldPosition);
  float rim = pow(1. - max(dot(normal, viewDirection), 0.), 3.);
  float daylight = smoothstep(-.12, .5, dot(normal, normalize(-v_worldPosition)));
  gl_FragColor = vec4(u_color * (1. + rim), rim * daylight * .65);
  #include <tonemapping_fragment>
  #include <colorspace_fragment>
}`

export function createSolarSystem(
  canvas: HTMLCanvasElement,
  onReady: () => void
) {
  const renderer = new THREE.WebGLRenderer({
    canvas,
    alpha: true,
    antialias: true,
    powerPreference: 'high-performance',
  })
  renderer.outputColorSpace = THREE.SRGBColorSpace
  renderer.toneMapping = THREE.ACESFilmicToneMapping
  renderer.toneMappingExposure = 0.9
  const scene = new THREE.Scene()
  const camera = new THREE.PerspectiveCamera(43, 1, 0.1, 300)
  const controls = new OrbitControls(camera, canvas)
  canvas.style.touchAction = 'pan-y'
  controls.enableDamping = true
  controls.dampingFactor = 0.07
  controls.enablePan = false
  controls.enableZoom = false
  controls.minDistance = 2
  controls.maxDistance = 75
  controls.maxPolarAngle = Math.PI * 0.82
  controls.minPolarAngle = 0.12
  scene.add(new THREE.AmbientLight(0x8cadd8, 0.12))
  scene.add(new THREE.HemisphereLight(0xb3d3ff, 0x202735, 0.7))
  const sunLight = new THREE.PointLight(0xfff2dc, 100, 0, 2)
  scene.add(sunLight)
  const loader = new THREE.TextureLoader()
  const textures: THREE.Texture[] = []
  const textureCache = new Map<string, THREE.Texture>()
  const loadTexture = (name: string): THREE.Texture => {
    const cached = textureCache.get(name)
    if (cached) return cached
    const texture = loader.load(`/textures/solar-system/${name}`, renderOnce)
    texture.colorSpace = THREE.SRGBColorSpace
    texture.anisotropy = Math.min(renderer.capabilities.getMaxAnisotropy(), 4)
    textures.push(texture)
    textureCache.set(name, texture)
    return texture
  }
  const galaxy = loadTexture('galaxy.jpg')
  galaxy.mapping = THREE.EquirectangularReflectionMapping
  scene.background = galaxy
  scene.backgroundIntensity = 1.1
  scene.backgroundRotation.set(0.3, 0.6, -0.35)
  const solar = new THREE.Group()
  solar.rotation.z = -0.18
  scene.add(solar)
  // Explicit triangles avoid gaps from indexed drawing on WebGL backends.
  const sphere = new THREE.SphereGeometry(1, 64, 40).toNonIndexed()
  const sun = new THREE.Mesh(
    sphere,
    new THREE.MeshBasicMaterial({
      map: loadTexture('sun.jpg'),
      color: new THREE.Color(2.0, 1.25, 0.5),
    })
  )
  sun.scale.setScalar(1.2)
  solar.add(sun)
  const coronaSize = 256
  const coronaPixels = new Uint8Array(coronaSize * coronaSize * 4)
  for (let i = 0; i < coronaSize * coronaSize; i++) {
    const x = (((i % coronaSize) + 0.5) / coronaSize) * 2 - 1
    const y = ((Math.floor(i / coronaSize) + 0.5) / coronaSize) * 2 - 1
    const radius = Math.hypot(x, y)
    const glow =
      Math.exp(-radius * radius * 10) *
      0.45 *
      (1 - THREE.MathUtils.smoothstep(radius, 0.75, 1))
    coronaPixels.set([255, 255, 255, Math.round(glow * 255)], i * 4)
  }
  const coronaTexture = new THREE.DataTexture(
    coronaPixels,
    coronaSize,
    coronaSize
  )
  coronaTexture.minFilter = THREE.LinearFilter
  coronaTexture.magFilter = THREE.LinearFilter
  coronaTexture.needsUpdate = true
  textures.push(coronaTexture)
  // Sprites face the camera without a triangle fan or manual rotation updates.
  const corona = new THREE.Sprite(
    new THREE.SpriteMaterial({
      map: coronaTexture,
      color: new THREE.Color(2, 0.75, 0.08),
      transparent: true,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
    })
  )
  corona.scale.set(8.4, 8.4, 1)
  scene.add(corona)
  const bodies = PLANETS.map((planet) => {
    const orbit = new THREE.Group()
    orbit.rotation.y = planet.phase
    solar.add(orbit)
    const mesh = new THREE.Mesh(
      sphere,
      new THREE.MeshStandardMaterial({
        map: loadTexture(`${planet.texture}.jpg`),
        roughness: 1,
        metalness: 0,
      })
    )
    mesh.scale.setScalar(planet.radius)
    mesh.position.x = planet.distance
    mesh.rotation.z = 0.2
    orbit.add(mesh)
    if (
      planet.name === 'Earth' ||
      planet.name === 'Uranus' ||
      planet.name === 'Neptune'
    ) {
      const atmosphere = new THREE.Mesh(
        sphere,
        new THREE.ShaderMaterial({
          vertexShader: ATMOSPHERE_VERTEX,
          fragmentShader: ATMOSPHERE_FRAGMENT,
          uniforms: { u_color: { value: new THREE.Color(0x53bfff) } },
          transparent: true,
          depthWrite: false,
          blending: THREE.AdditiveBlending,
        })
      )
      atmosphere.scale.setScalar(1.035)
      mesh.add(atmosphere)
    }
    const points: THREE.Vector3[] = []
    for (let i = 0; i <= 160; i++) {
      const angle = (i / 160) * Math.PI * 2
      points.push(
        new THREE.Vector3(
          Math.cos(angle) * planet.distance,
          0,
          Math.sin(angle) * planet.distance
        )
      )
    }
    solar.add(
      new THREE.Line(
        new THREE.BufferGeometry().setFromPoints(points),
        new THREE.LineBasicMaterial({
          color: 0x7b99bc,
          transparent: true,
          opacity: 0.12,
        })
      )
    )
    return { planet, orbit, mesh, systemRadius: Number(planet.radius) }
  })
  const moonSphere = new THREE.SphereGeometry(1, 32, 20).toNonIndexed()
  const moons = bodies.flatMap((body) => {
    const system = SATELLITE_SYSTEMS[body.planet.name]
    if (!system) return []
    const anchor = new THREE.Group()
    anchor.position.copy(body.mesh.position)
    anchor.rotation.x = system.inclination
    body.orbit.add(anchor)
    return system.moons.map((satellite, index) => {
      const orbit = new THREE.Group()
      const phase = index * 2.399 + 0.35
      orbit.rotation.y = phase
      anchor.add(orbit)
      const mesh = new THREE.Mesh(
        moonSphere,
        new THREE.MeshLambertMaterial({
          map: loadTexture(`moons/${satellite.name.toLowerCase()}.jpg`),
        })
      )
      mesh.name = satellite.name
      mesh.scale.setScalar(satellite.radius)
      if (satellite.name === 'Phobos' || satellite.name === 'Deimos') {
        mesh.scale.multiply(new THREE.Vector3(1.3, 0.85, 0.9))
      }
      mesh.position.x = satellite.distance
      orbit.add(mesh)
      const points: THREE.Vector3[] = []
      for (let i = 0; i <= 64; i++) {
        const angle = (i / 64) * Math.PI * 2
        points.push(
          new THREE.Vector3(
            Math.cos(angle) * satellite.distance,
            0,
            Math.sin(angle) * satellite.distance
          )
        )
      }
      const track = new THREE.Line(
        new THREE.BufferGeometry().setFromPoints(points),
        new THREE.LineBasicMaterial({
          color: 0xa4b6ca,
          transparent: true,
          opacity: 0.18,
        })
      )
      track.visible = false
      anchor.add(track)
      body.systemRadius = Math.max(
        body.systemRadius,
        satellite.distance + satellite.radius
      )
      return { parent: body, satellite, orbit, mesh, track, phase }
    })
  })
  const saturn = bodies[5].mesh
  const ringGeometry = new THREE.RingGeometry(1.35, 2.25, 96)
  const ringPositions = ringGeometry.getAttribute('position')
  const ringUV = ringGeometry.getAttribute('uv')
  for (let i = 0; i < ringPositions.count; i++) {
    ringUV.setXY(
      i,
      (Math.hypot(ringPositions.getX(i), ringPositions.getY(i)) - 1.35) / 0.9,
      0.5
    )
  }
  const ring = new THREE.Mesh(
    ringGeometry,
    new THREE.MeshStandardMaterial({
      map: loadTexture('saturn-ring.png'),
      side: THREE.DoubleSide,
      transparent: true,
      opacity: 0.9,
      roughness: 1,
    })
  )
  ring.rotation.x = Math.PI / 2
  saturn.add(ring)
  saturn.rotation.z = 0.47

  const starPositions = new Float32Array(4000 * 3)
  const starSizes = new Float32Array(4000)
  const starColors = new Float32Array(4000 * 3)
  let seed = 4317
  const random = (): number => {
    seed = (seed * 16807) % 2147483647
    return (seed - 1) / 2147483646
  }
  for (let i = 0; i < starSizes.length; i++) {
    const angle = random() * Math.PI * 2,
      vertical = random() * 2 - 1,
      radius = 65 + random() * 65
    const horizontal = Math.sqrt(1 - vertical * vertical)
    starPositions.set(
      [
        Math.cos(angle) * horizontal * radius,
        vertical * radius,
        Math.sin(angle) * horizontal * radius,
      ],
      i * 3
    )
    starSizes[i] = 1 + random() * 2.3
    const tint = random()
    let color = [0.85, 0.91, 1]
    if (tint < 0.12) color = [1, 0.66, 0.38]
    else if (tint < 0.3) color = [0.45, 0.78, 1]
    starColors.set(color, i * 3)
  }
  const starGeometry = new THREE.BufferGeometry()
  starGeometry.setAttribute(
    'position',
    new THREE.BufferAttribute(starPositions, 3)
  )
  starGeometry.setAttribute('a_size', new THREE.BufferAttribute(starSizes, 1))
  starGeometry.setAttribute('a_color', new THREE.BufferAttribute(starColors, 3))
  const starUniforms = {
    u_pointer: { value: new THREE.Vector2(-10, -10) },
    u_strength: { value: 0 },
    u_pixelRatio: { value: 1 },
  }
  const starMaterial = new THREE.ShaderMaterial({
    vertexShader: STAR_VERTEX,
    fragmentShader: STAR_FRAGMENT,
    uniforms: starUniforms,
    transparent: true,
    depthWrite: false,
    blending: THREE.AdditiveBlending,
  })
  scene.add(new THREE.Points(starGeometry, starMaterial))
  const pointer = new THREE.Vector2(-10, -10)
  let pointerActive = 0
  let paused = false
  let visible = true
  let disposed = false
  let frame = 0
  let previous = 0
  let elapsed = 0
  let focusIndex = -1
  let cameraMove: {
    startedAt: number
    fromPosition: THREE.Vector3
    fromTarget: THREE.Vector3
    target: THREE.Vector3
    offset: THREE.Vector3
  } | null = null
  const focusTarget = new THREE.Vector3()
  const destinationPosition = new THREE.Vector3()
  const reduce = matchMedia('(prefers-reduced-motion: reduce)')

  function renderOnce(): void {
    if (disposed) return
    renderer.render(scene, camera)
  }
  function navigateCamera(
    position: THREE.Vector3,
    target: THREE.Vector3,
    index: number,
    animated = true
  ): void {
    focusIndex = index
    for (const moon of moons) {
      moon.track.visible = index >= 0 && moon.parent === bodies[index]
    }
    if (!animated || reduce.matches) {
      cameraMove = null
      camera.position.copy(position)
      controls.target.copy(target)
      controls.update()
      renderOnce()
      return
    }
    cameraMove = {
      startedAt: performance.now(),
      fromPosition: camera.position.clone(),
      fromTarget: controls.target.clone(),
      target,
      offset: position.clone().sub(target),
    }
    resume()
  }
  function reset(animated = true): void {
    const mobile = canvas.clientWidth < 640
    navigateCamera(
      new THREE.Vector3(mobile ? 8 : 2, mobile ? 24 : 17, mobile ? 38 : 29),
      new THREE.Vector3(),
      -1,
      animated
    )
  }
  function draw(now: number): void {
    frame = 0
    if (disposed || !visible || document.hidden) return
    const delta = previous ? Math.min((now - previous) / 1000, 0.05) : 0
    previous = now
    if (!paused && !reduce.matches) {
      elapsed += delta
      sun.rotation.y += delta * 0.04
      for (const body of bodies) {
        body.orbit.rotation.y =
          body.planet.phase + elapsed * body.planet.speed * 0.12
        body.mesh.rotation.y += delta * 0.09
      }
      for (const moon of moons) {
        moon.orbit.rotation.y =
          moon.phase + elapsed * moon.satellite.speed * 0.25
      }
    }
    if (cameraMove) {
      const progress = reduce.matches
        ? 1
        : Math.min((now - cameraMove.startedAt) / 950, 1)
      const eased = progress * progress * (3 - 2 * progress)
      focusTarget.copy(cameraMove.target)
      if (focusIndex >= 0) {
        bodies[focusIndex].mesh.getWorldPosition(focusTarget)
      }
      destinationPosition.copy(focusTarget).add(cameraMove.offset)
      camera.position.lerpVectors(
        cameraMove.fromPosition,
        destinationPosition,
        eased
      )
      controls.target.lerpVectors(cameraMove.fromTarget, focusTarget, eased)
      if (progress === 1) cameraMove = null
    } else if (focusIndex >= 0) {
      bodies[focusIndex].mesh.getWorldPosition(focusTarget)
      camera.position.add(focusTarget).sub(controls.target)
      controls.target.copy(focusTarget)
    }
    controls.dampingFactor = 1 - Math.exp(-delta * 4.5)
    controls.update()
    const smoothing = 1 - Math.exp(-delta * 9)
    starUniforms.u_pointer.value.lerp(pointer, smoothing)
    starUniforms.u_strength.value +=
      (pointerActive - starUniforms.u_strength.value) * smoothing
    renderOnce()
    if (!reduce.matches) frame = requestAnimationFrame(draw)
  }
  function resume(): void {
    cancelAnimationFrame(frame)
    previous = 0
    draw(performance.now())
  }
  const resize = new ResizeObserver(() => {
    const width = canvas.clientWidth,
      height = canvas.clientHeight
    const ratio = Math.min(
      devicePixelRatio || 1,
      width < 640 ? 1 : 1.25,
      Math.sqrt(3_000_000 / (width * height))
    )
    renderer.setPixelRatio(ratio)
    renderer.setSize(width, height, false)
    camera.aspect = width / height
    camera.updateProjectionMatrix()
    starUniforms.u_pixelRatio.value = ratio
    if (focusIndex === -1) reset(false)
    resume()
    canvas.dataset.ready = 'true'
    onReady()
  })
  resize.observe(canvas)
  const observer = new IntersectionObserver(([entry]) => {
    visible = entry.isIntersecting
    resume()
  })
  observer.observe(canvas)
  function move(event: PointerEvent): void {
    const bounds = canvas.getBoundingClientRect()
    pointer.set(
      ((event.clientX - bounds.left) / bounds.width) * 2 - 1,
      (-(event.clientY - bounds.top) / bounds.height) * 2 + 1
    )
    pointerActive = event.pointerType === 'mouse' ? 1 : 0
    if (reduce.matches) {
      starUniforms.u_pointer.value.copy(pointer)
      starUniforms.u_strength.value = pointerActive
      renderOnce()
    }
  }
  function leave(): void {
    pointerActive = 0
    if (reduce.matches) {
      starUniforms.u_strength.value = 0
      renderOnce()
    }
  }
  function wheel(event: WheelEvent): void {
    if (!event.ctrlKey) return
    event.preventDefault()
    cameraMove = null
    const offset = camera.position.clone().sub(controls.target)
    const distance = THREE.MathUtils.clamp(
      offset.length() * Math.exp(event.deltaY * 0.002),
      2,
      75
    )
    camera.position.copy(controls.target).add(offset.setLength(distance))
    renderOnce()
  }
  function interruptCamera(): void {
    cameraMove = null
  }
  function controlsChanged(): void {
    if (reduce.matches) renderOnce()
  }
  canvas.addEventListener('pointermove', move)
  canvas.addEventListener('pointerleave', leave)
  canvas.addEventListener('wheel', wheel, { passive: false })
  controls.addEventListener('change', controlsChanged)
  controls.addEventListener('start', interruptCamera)
  document.addEventListener('visibilitychange', resume)
  reduce.addEventListener('change', resume)
  reset(false)
  return {
    setPaused(value: boolean) {
      paused = value
      resume()
    },
    focus(index: number) {
      const body = index < 0 ? sun : bodies[index].mesh
      const radius = index < 0 ? 1.2 : bodies[index].planet.radius
      const target = body.getWorldPosition(new THREE.Vector3())
      const hasMoons = index >= 0 && bodies[index].systemRadius > radius
      const viewRadius = hasMoons ? bodies[index].systemRadius : radius
      navigateCamera(
        target
          .clone()
          .add(
            new THREE.Vector3(
              0,
              viewRadius * (hasMoons ? 1.4 : 3),
              viewRadius * (hasMoons ? 3.5 : 7)
            )
          ),
        target,
        index < 0 ? -2 : index
      )
    },
    reset,
    zoom(scale: number) {
      const offset = camera.position.clone().sub(controls.target)
      navigateCamera(
        controls.target
          .clone()
          .add(
            offset.setLength(
              THREE.MathUtils.clamp(offset.length() * scale, 2, 75)
            )
          ),
        controls.target.clone(),
        focusIndex
      )
    },
    dispose() {
      disposed = true
      cancelAnimationFrame(frame)
      resize.disconnect()
      observer.disconnect()
      canvas.removeEventListener('pointermove', move)
      canvas.removeEventListener('pointerleave', leave)
      canvas.removeEventListener('wheel', wheel)
      document.removeEventListener('visibilitychange', resume)
      reduce.removeEventListener('change', resume)
      controls.removeEventListener('change', controlsChanged)
      controls.removeEventListener('start', interruptCamera)
      controls.dispose()
      scene.traverse((object) => {
        if (
          object instanceof THREE.Mesh ||
          object instanceof THREE.Line ||
          object instanceof THREE.Points
        ) {
          object.geometry.dispose()
          const materials = Array.isArray(object.material)
            ? object.material
            : [object.material]
          for (const material of materials) material.dispose()
        }
      })
      corona.material.dispose()
      for (const texture of textures) texture.dispose()
      renderer.dispose()
      delete canvas.dataset.ready
    },
  }
}
