// Dashboard central del harness 528hz: refresca el fragmento (proyectos → sesiones → árbol + carriles) cada 2 s.
// Recuerda la ventana, el filtro de proyecto y lo que el humano plegó o desplegó. Sin librerías externas.
(() => {
  const vista = document.getElementById('vista')
  const estado = document.getElementById('estado')

  const leer = (clave, defecto) => {
    try {
      return localStorage.getItem(clave) ?? defecto
    } catch (_) {
      return defecto // sin almacenamiento local, vale el valor por defecto
    }
  }
  const guardar = (clave, valor) => {
    try {
      localStorage.setItem(clave, valor)
    } catch (_) {
      // preferencia solo de esta pestaña
    }
  }

  let ventana = leer('hz-ventana', '10m')
  let proyecto = leer('hz-proyecto', '')
  let plegados = {}
  try {
    plegados = JSON.parse(leer('hz-plegados', '{}')) || {}
  } catch (_) {
    plegados = {}
  }

  const marcarVentana = () => {
    for (const b of document.querySelectorAll('[data-ventana]')) {
      b.setAttribute('aria-pressed', String(b.dataset.ventana === ventana))
    }
  }

  // Lo que el humano plegó o desplegó a mano gana sobre lo que propone el servidor.
  const aplicarPlegados = () => {
    for (const d of vista.querySelectorAll('details[data-plegable]')) {
      const abierto = plegados[d.dataset.plegable]
      if (typeof abierto === 'boolean' && d.open !== abierto) d.open = abierto
    }
  }

  async function refrescar() {
    try {
      const q = 'ventana=' + encodeURIComponent(ventana) + '&proyecto=' + encodeURIComponent(proyecto)
      const r = await fetch('/fragmento?' + q, { cache: 'no-store' })
      if (!r.ok) throw new Error('HTTP ' + r.status)
      vista.innerHTML = await r.text()
      aplicarPlegados()
      estado.textContent = 'actualizado ' + new Date().toLocaleTimeString()
      estado.dataset.ok = 'true'
    } catch (err) {
      estado.textContent = 'sin conexión con hz dashboard (' + err.message + ')'
      estado.dataset.ok = 'false'
    }
  }

  for (const b of document.querySelectorAll('[data-ventana]')) {
    b.addEventListener('click', () => {
      ventana = b.dataset.ventana
      guardar('hz-ventana', ventana)
      marcarVentana()
      refrescar()
    })
  }

  // El fragmento se reemplaza cada 2 s: los clics se escuchan en el contenedor. Solo un clic del humano en la
  // cabecera de un proyecto cuenta como plegado (no el open que propone el servidor al dibujar).
  vista.addEventListener('click', e => {
    const s = e.target.closest('summary')
    if (s && s.parentElement && s.parentElement.dataset.plegable) {
      plegados[s.parentElement.dataset.plegable] = !s.parentElement.open
      guardar('hz-plegados', JSON.stringify(plegados))
      return
    }
    const b = e.target.closest('[data-proyecto]')
    if (!b) return
    proyecto = b.dataset.proyecto
    guardar('hz-proyecto', proyecto)
    refrescar()
  })

  marcarVentana()
  refrescar()
  setInterval(() => {
    if (!document.hidden) refrescar()
  }, 2000)
})()
