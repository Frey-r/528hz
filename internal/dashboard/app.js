// Dashboard del harness 528hz: refresca el fragmento (árbol + carriles) cada 2 s. Sin librerías externas.
(() => {
  const vista = document.getElementById('vista')
  const estado = document.getElementById('estado')
  let ventana = '10m'
  try {
    ventana = localStorage.getItem('hz-ventana') || ventana
  } catch (_) {
    // sin almacenamiento local, la ventana vuelve a 10 min
  }

  const marcar = () => {
    for (const b of document.querySelectorAll('[data-ventana]')) {
      b.setAttribute('aria-pressed', String(b.dataset.ventana === ventana))
    }
  }

  async function refrescar() {
    try {
      const r = await fetch('/fragmento?ventana=' + encodeURIComponent(ventana), { cache: 'no-store' })
      if (!r.ok) throw new Error('HTTP ' + r.status)
      vista.innerHTML = await r.text()
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
      try {
        localStorage.setItem('hz-ventana', ventana)
      } catch (_) {
        // preferencia solo de esta sesión
      }
      marcar()
      refrescar()
    })
  }
  marcar()
  refrescar()
  setInterval(() => {
    if (!document.hidden) refrescar()
  }, 2000)
})()
