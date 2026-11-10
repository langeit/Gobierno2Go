package handlers

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"Gobierno2Go/internal/models"
	"Gobierno2Go/internal/repository"
	"Gobierno2Go/internal/session"
)

func (h *AdquisicionesHandler) usuarioID(r *http.Request) string {
	info, ok := session.FromContext(r.Context())
	if !ok {
		return ""
	}
	return info.UserID
}

func mensajeOperacion(err error) string {
	if errors.Is(err, repository.ErrDatos) {
		const prefijo = "los datos enviados no son válidos: "
		if msg, ok := strings.CutPrefix(err.Error(), prefijo); ok {
			return msg
		}
		return "Revise los datos. Hay un campo obligatorio vacío o un valor que no corresponde."
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "El registro no existe."
	}
	return "No se pudo completar la operación."
}

func entero(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func decimal(s string) float64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	n, _ := strconv.ParseFloat(s, 64)
	return n
}

func (h *AdquisicionesHandler) catalogoOrden(data map[string]interface{}) error {
	materiales, err := h.repo.MaterialesCatalogo(0)
	if err != nil {
		return err
	}
	asignaciones, err := h.repo.AsignacionesMantenedor()
	if err != nil {
		return err
	}
	proveedores, err := h.repo.Proveedores()
	if err != nil {
		return err
	}
	tipos, err := h.repo.DominiosPorTipo(models.CodTipoOrden)
	if err != nil {
		return err
	}
	impuestos, err := h.repo.DominiosPorTipo(models.CodTipoImpuesto)
	if err != nil {
		return err
	}
	data["Materiales"] = materiales
	data["Asignaciones"] = asignaciones
	data["Proveedores"] = proveedores
	data["TiposOrden"] = tipos
	data["Impuestos"] = impuestos
	return nil
}

func leerOrden(r *http.Request, usuario string) models.OrdenEntrada {
	in := models.OrdenEntrada{
		Numero:            entero(r.FormValue("numero")),
		TipoOrden:         entero(r.FormValue("tipo_orden")),
		RutProveedor:      r.FormValue("proveedor"),
		IDUsuario:         usuario,
		ModoOrden:         entero(r.FormValue("modo")),
		TipoImpuesto:      entero(r.FormValue("impuesto")),
		DescServicio:      strings.TrimSpace(r.FormValue("desc_servicio")),
		CantServicio:      decimal(r.FormValue("cant_servicio")),
		ValorUnitServicio: decimal(r.FormValue("valor_servicio")),
	}
	mats := r.Form["linea_material"]
	cants := r.Form["linea_cantidad"]
	vals := r.Form["linea_valor"]
	for i := range mats {
		cod := entero(mats[i])
		if cod == 0 {
			continue
		}
		cant, val := 0, 0
		if i < len(cants) {
			cant = entero(cants[i])
		}
		if i < len(vals) {
			val = entero(vals[i])
		}
		in.Lineas = append(in.Lineas, models.LineaCantidad{CodMaterial: cod, Cantidad: cant, ValorUnitario: val})
	}
	asigs := r.Form["imp_asig"]
	montos := r.Form["imp_monto"]
	for i := range asigs {
		corr := entero(asigs[i])
		if corr == 0 {
			continue
		}
		monto := 0.0
		if i < len(montos) {
			monto = decimal(montos[i])
		}
		in.Imputaciones = append(in.Imputaciones, models.ImputacionLinea{CorrAsig: corr, Monto: monto})
	}
	return in
}

func padLineas(src []models.LineaCantidad, n int) []models.LineaCantidad {
	out := append([]models.LineaCantidad{}, src...)
	for len(out) < n {
		out = append(out, models.LineaCantidad{})
	}
	return out
}

func padImputaciones(src []models.ImputacionLinea, n int) []models.ImputacionLinea {
	out := append([]models.ImputacionLinea{}, src...)
	for len(out) < n {
		out = append(out, models.ImputacionLinea{})
	}
	return out
}

func (h *AdquisicionesHandler) pintarOrden(w http.ResponseWriter, r *http.Request, in models.OrdenEntrada, facturadas []models.LineaCantidad, titulo, accion, errMsg string) {
	data := pageData(r)
	data["Title"] = titulo
	data["Seccion"] = "ordenes"
	data["Accion"] = accion
	data["Orden"] = in
	data["Facturadas"] = facturadas
	data["Lineas"] = padLineas(in.Lineas, 5)
	data["Imputaciones"] = padImputaciones(in.Imputaciones, 3)
	if errMsg != "" {
		data["Error"] = errMsg
	}
	if err := h.catalogoOrden(data); err != nil {
		data["Error"] = "No se pudieron cargar los catálogos."
	}
	h.render(w, "adq_orden_form.html", data)
}

func (h *AdquisicionesHandler) OrdenesPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Órdenes de compra"
	data["Seccion"] = "ordenes"
	h.flash(data, r.URL.Query().Get("ok"))
	ordenes, err := h.repo.Ordenes()
	if err != nil {
		data["Error"] = "No se pudo leer el listado de órdenes."
	}
	data["Ordenes"] = ordenes
	h.render(w, "adq_ordenes.html", data)
}

func (h *AdquisicionesHandler) OrdenNuevaPage(w http.ResponseWriter, r *http.Request) {
	modo := entero(r.URL.Query().Get("modo"))
	if modo != models.ModoOrdenServicios {
		modo = models.ModoOrdenMateriales
	}
	sig, _ := h.repo.SiguienteOrden()
	titulo := "Ingreso de orden de materiales"
	if modo == models.ModoOrdenServicios {
		titulo = "Ingreso de orden de servicios"
	}
	h.pintarOrden(w, r, models.OrdenEntrada{Numero: sig, ModoOrden: modo}, nil, titulo, "/adquisiciones/ordenes/nueva", "")
}

func (h *AdquisicionesHandler) OrdenNuevaPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulario inválido", http.StatusBadRequest)
		return
	}
	in := leerOrden(r, h.usuarioID(r))
	if err := h.repo.CrearOrden(in); err != nil {
		titulo := "Ingreso de orden de materiales"
		if in.ModoOrden == models.ModoOrdenServicios {
			titulo = "Ingreso de orden de servicios"
		}
		h.pintarOrden(w, r, in, nil, titulo, "/adquisiciones/ordenes/nueva", mensajeOperacion(err))
		return
	}
	h.redirigir(w, r, "/adquisiciones/ordenes/"+strconv.Itoa(in.Numero), "creado")
}

func (h *AdquisicionesHandler) OrdenVerPage(w http.ResponseWriter, r *http.Request, nro int) {
	h.pintarOrdenVer(w, r, nro, false, "", "")
}

func (h *AdquisicionesHandler) pintarOrdenVer(w http.ResponseWriter, r *http.Request, nro int, documento bool, tituloDoc, volver string) {
	data := pageData(r)
	data["Title"] = "Orden de compra"
	data["Seccion"] = "ordenes"
	if documento {
		data["Title"] = tituloDoc
		data["Seccion"] = "reportes"
		data["Documento"] = true
		data["Volver"] = volver
	}
	h.flash(data, r.URL.Query().Get("ok"))
	if nro > 0 {
		orden, err := h.repo.Orden(nro)
		if err != nil {
			data["Error"] = mensajeOperacion(err)
		} else if documento && orden.TipoOrden != entero(r.URL.Query().Get("tipo_esperado")) && r.URL.Query().Get("tipo_esperado") != "" {
			data["Error"] = "No existe un documento de ese tipo con el número " + strconv.Itoa(nro) + "."
		} else {
			data["Orden"] = orden
			if !documento {
				data["Title"] = "Orden " + strconv.Itoa(orden.Numero)
			}
		}
	}
	h.render(w, "adq_orden_ver.html", data)
}

func (h *AdquisicionesHandler) OrdenEditarPage(w http.ResponseWriter, r *http.Request, nro int) {
	orden, err := h.repo.Orden(nro)
	if err != nil {
		h.redirigir(w, r, "/adquisiciones/ordenes", "noencontrado")
		return
	}
	if !orden.Editable {
		h.pintarOrdenVer(w, r, nro, false, "", "")
		return
	}
	h.pintarOrden(w, r, entradaDesdeOrden(orden), facturadas(orden), "Modificar orden "+strconv.Itoa(nro), "/adquisiciones/ordenes/"+strconv.Itoa(nro), "")
}

func entradaDesdeOrden(o models.OrdenCompra) models.OrdenEntrada {
	in := models.OrdenEntrada{
		Numero: o.Numero, TipoOrden: o.TipoOrden, RutProveedor: o.RutProveedor,
		ModoOrden: o.ModoOrden, TipoImpuesto: o.TipoImpuesto,
		DescServicio: o.DescServicio, CantServicio: o.CantServicio, ValorUnitServicio: o.ValorUnitServicio,
	}
	for _, l := range o.Lineas {
		if !l.Facturado {
			in.Lineas = append(in.Lineas, l)
		}
	}
	in.Imputaciones = o.Imputaciones
	return in
}

func facturadas(o models.OrdenCompra) []models.LineaCantidad {
	var out []models.LineaCantidad
	for _, l := range o.Lineas {
		if l.Facturado {
			out = append(out, l)
		}
	}
	return out
}

func (h *AdquisicionesHandler) OrdenEditarPost(w http.ResponseWriter, r *http.Request, nro int) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulario inválido", http.StatusBadRequest)
		return
	}
	in := leerOrden(r, h.usuarioID(r))
	in.Numero = nro
	if err := h.repo.ActualizarOrden(in); err != nil {
		orden, _ := h.repo.Orden(nro)
		h.pintarOrden(w, r, in, facturadas(orden), "Modificar orden "+strconv.Itoa(nro), "/adquisiciones/ordenes/"+strconv.Itoa(nro), mensajeOperacion(err))
		return
	}
	h.redirigir(w, r, "/adquisiciones/ordenes/"+strconv.Itoa(nro), "actualizado")
}

func (h *AdquisicionesHandler) OrdenEliminarPost(w http.ResponseWriter, r *http.Request, nro int) {
	if err := h.repo.EliminarOrden(nro); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.redirigir(w, r, "/adquisiciones/ordenes", "noencontrado")
			return
		}
		data := pageData(r)
		data["Title"] = "Orden de compra"
		data["Seccion"] = "ordenes"
		data["Error"] = mensajeOperacion(err)
		if orden, errOrden := h.repo.Orden(nro); errOrden == nil {
			data["Orden"] = orden
		}
		h.render(w, "adq_orden_ver.html", data)
		return
	}
	h.redirigir(w, r, "/adquisiciones/ordenes", "eliminado")
}

// ---------- Facturas ----------

func (h *AdquisicionesHandler) FacturasPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Ingreso de facturas"
	data["Seccion"] = "facturas"
	h.flash(data, r.URL.Query().Get("ok"))
	facturas, err := h.repo.Facturas()
	if err != nil {
		data["Error"] = "No se pudo leer el listado de facturas."
	}
	data["Facturas"] = facturas
	h.render(w, "adq_facturas.html", data)
}

func (h *AdquisicionesHandler) FacturaNuevaPage(w http.ResponseWriter, r *http.Request) {
	h.pintarFactura(w, r, models.FacturaEntrada{}, "")
}

func (h *AdquisicionesHandler) pintarFactura(w http.ResponseWriter, r *http.Request, in models.FacturaEntrada, errMsg string) {
	data := pageData(r)
	data["Title"] = "Ingreso de factura"
	data["Seccion"] = "facturas"
	modo := entero(r.FormValue("modo"))
	if modo == 0 {
		modo = entero(r.URL.Query().Get("modo"))
	}
	if modo != models.ModoFacturaServicios {
		modo = models.ModoFacturaMateriales
	}
	if in.Modo == 0 {
		in.Modo = modo
	}
	rut := r.FormValue("proveedor")
	if rut == "" {
		rut = r.URL.Query().Get("proveedor")
	}
	nro := in.NroOrden
	if nro == 0 {
		nro = entero(r.URL.Query().Get("orden"))
	}
	ordenes, err := h.repo.OrdenesPorFacturar(in.Modo, rut)
	if err != nil && errMsg == "" {
		errMsg = "No se pudieron leer las órdenes pendientes."
	}
	proveedores, errP := h.repo.Proveedores()
	impuestos, errI := h.repo.DominiosPorTipo(models.CodTipoImpuesto)
	if (errP != nil || errI != nil) && errMsg == "" {
		errMsg = "No se pudieron cargar los catálogos."
	}
	data["Entrada"] = in
	data["Modo"] = in.Modo
	data["ProveedorSel"] = rut
	data["Ordenes"] = ordenes
	data["Proveedores"] = proveedores
	data["Impuestos"] = impuestos
	if nro > 0 {
		orden, errO := h.repo.Orden(nro)
		if errO != nil && errMsg == "" {
			errMsg = mensajeOperacion(errO)
		} else if errO == nil {
			data["Orden"] = orden
			if in.TipoImpuesto == 0 {
				in.TipoImpuesto = orden.TipoImpuesto
				data["Entrada"] = in
			}
			var pend []models.LineaCantidad
			for _, l := range orden.Lineas {
				if !l.Facturado {
					pend = append(pend, l)
				}
			}
			data["Pendientes"] = pend
		}
	}
	if errMsg != "" {
		data["Error"] = errMsg
	}
	h.render(w, "adq_factura_form.html", data)
}

func (h *AdquisicionesHandler) FacturaNuevaPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulario inválido", http.StatusBadRequest)
		return
	}
	in := models.FacturaEntrada{
		NroOrden:          entero(r.FormValue("orden")),
		NroFactura:        entero(r.FormValue("nro_factura")),
		FechaFactura:      r.FormValue("fecha_factura"),
		Modo:              entero(r.FormValue("modo")),
		TipoImpuesto:      entero(r.FormValue("impuesto")),
		IDUsuario:         h.usuarioID(r),
		DescServicio:      strings.TrimSpace(r.FormValue("desc_servicio")),
		CantServicio:      decimal(r.FormValue("cant_servicio")),
		ValorUnitServicio: decimal(r.FormValue("valor_servicio")),
	}
	for _, c := range r.Form["factura_corr"] {
		corr := entero(c)
		if corr == 0 {
			continue
		}
		in.Lineas = append(in.Lineas, models.LineaCantidad{
			Correlativo:   corr,
			CodMaterial:   entero(r.FormValue("fac_mat_" + c)),
			Cantidad:      entero(r.FormValue("fac_cant_" + c)),
			ValorUnitario: entero(r.FormValue("fac_val_" + c)),
		})
	}
	id, err := h.repo.CrearFactura(in)
	if err != nil {
		h.pintarFactura(w, r, in, mensajeOperacion(err))
		return
	}
	h.redirigir(w, r, "/adquisiciones/facturas/"+strconv.Itoa(id), "facturado")
}

func (h *AdquisicionesHandler) FacturaVerPage(w http.ResponseWriter, r *http.Request, id int) {
	data := pageData(r)
	data["Title"] = "Factura"
	data["Seccion"] = "facturas"
	h.flash(data, r.URL.Query().Get("ok"))
	factura, err := h.repo.Factura(id)
	if err != nil {
		data["Error"] = mensajeOperacion(err)
	} else {
		data["Factura"] = factura
		data["Title"] = "Factura " + strconv.Itoa(factura.NroFactura)
	}
	h.render(w, "adq_factura_ver.html", data)
}

// ---------- Solicitudes ----------

func (h *AdquisicionesHandler) SolicitudesPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Solicitudes de materiales"
	data["Seccion"] = "solicitudes"
	h.flash(data, r.URL.Query().Get("ok"))
	h.render(w, "adq_solicitudes.html", data)
}

func (h *AdquisicionesHandler) SolicitudNuevaPage(w http.ResponseWriter, r *http.Request) {
	h.pintarSolicitud(w, r, models.SolicitudEntrada{}, "")
}

func (h *AdquisicionesHandler) pintarSolicitud(w http.ResponseWriter, r *http.Request, in models.SolicitudEntrada, errMsg string) {
	data := pageData(r)
	data["Title"] = "Nueva solicitud de materiales"
	data["Seccion"] = "solicitudes"
	depto, nombre, err := h.repo.DepartamentoUsuario(h.usuarioID(r))
	if err != nil && errMsg == "" {
		errMsg = "No se pudo leer el departamento del usuario."
	}
	deptos, errD := h.repo.Departamentos()
	materiales, errM := h.repo.MaterialesCatalogo(0)
	if (errD != nil || errM != nil) && errMsg == "" {
		errMsg = "No se pudieron cargar los catálogos."
	}
	if in.IDDepto == 0 {
		in.IDDepto = depto
	}
	data["Entrada"] = in
	data["DeptoFijo"] = depto
	data["DeptoNombre"] = nombre
	data["Departamentos"] = deptos
	data["Materiales"] = materiales
	data["Lineas"] = padLineas(in.Lineas, 5)
	if errMsg != "" {
		data["Error"] = errMsg
	}
	h.render(w, "adq_solicitud_form.html", data)
}

func leerLineasSolicitud(r *http.Request) []models.LineaCantidad {
	mats := r.Form["linea_material"]
	cants := r.Form["linea_cantidad"]
	var out []models.LineaCantidad
	for i := range mats {
		cod := entero(mats[i])
		if cod == 0 {
			continue
		}
		cant := 0
		if i < len(cants) {
			cant = entero(cants[i])
		}
		out = append(out, models.LineaCantidad{CodMaterial: cod, Cantidad: cant})
	}
	return out
}

func (h *AdquisicionesHandler) SolicitudNuevaPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulario inválido", http.StatusBadRequest)
		return
	}
	uid := h.usuarioID(r)
	depto, _, _ := h.repo.DepartamentoUsuario(uid)
	if depto == 0 {
		depto = entero(r.FormValue("depto"))
	}
	in := models.SolicitudEntrada{IDUsuario: uid, IDDepto: depto, Lineas: leerLineasSolicitud(r)}
	id, err := h.repo.CrearSolicitud(in)
	if err != nil {
		h.pintarSolicitud(w, r, in, mensajeOperacion(err))
		return
	}
	h.redirigir(w, r, "/adquisiciones/solicitudes/"+strconv.Itoa(id), "creado")
}

func (h *AdquisicionesHandler) SolicitudesEstadoPage(w http.ResponseWriter, r *http.Request, estado int, titulo, bajada string) {
	data := pageData(r)
	data["Title"] = titulo
	data["Bajada"] = bajada
	data["Seccion"] = "solicitudes"
	h.flash(data, r.URL.Query().Get("ok"))
	f := models.FiltroReporte{
		Depto:  entero(r.URL.Query().Get("depto")),
		Fecha1: r.URL.Query().Get("fecha1"),
		Fecha2: r.URL.Query().Get("fecha2"),
	}
	lista, err := h.repo.Solicitudes(estado, "", f)
	if err != nil {
		data["Error"] = mensajeOperacion(err)
	}
	deptos, _ := h.repo.Departamentos()
	data["Solicitudes"] = lista
	data["Departamentos"] = deptos
	data["Filtro"] = f
	data["MuestraFechas"] = estado == models.EstadoSolResuelta
	h.render(w, "adq_solicitud_lista.html", data)
}

func (h *AdquisicionesHandler) SolicitudVerPage(w http.ResponseWriter, r *http.Request, id int) {
	data := pageData(r)
	data["Title"] = "Solicitud de materiales"
	data["Seccion"] = "solicitudes"
	h.flash(data, r.URL.Query().Get("ok"))
	sol, err := h.repo.Solicitud(id)
	if err != nil {
		data["Error"] = mensajeOperacion(err)
	} else {
		data["Solicitud"] = sol
		data["Title"] = "Solicitud " + strconv.Itoa(sol.Numero)
	}
	h.render(w, "adq_solicitud_ver.html", data)
}

func (h *AdquisicionesHandler) SolicitudAprobarPage(w http.ResponseWriter, r *http.Request, id int) {
	h.pintarAprobacion(w, r, id, nil, true, "")
}

func (h *AdquisicionesHandler) pintarAprobacion(w http.ResponseWriter, r *http.Request, id int, lineas []models.LineaCantidad, visto bool, errMsg string) {
	data := pageData(r)
	data["Title"] = "Aprobar solicitud"
	data["Seccion"] = "solicitudes"
	sol, err := h.repo.Solicitud(id)
	if err != nil {
		data["Error"] = mensajeOperacion(err)
		h.render(w, "adq_solicitud_aprobar.html", data)
		return
	}
	if lineas == nil {
		lineas = sol.Lineas
	}
	materiales, errM := h.repo.MaterialesCatalogo(0)
	if errM != nil && errMsg == "" {
		errMsg = "No se pudieron cargar los materiales."
	}
	data["Solicitud"] = sol
	data["Lineas"] = padLineas(lineas, 5)
	data["Materiales"] = materiales
	data["VistoBueno"] = visto
	if errMsg != "" {
		data["Error"] = errMsg
	}
	h.render(w, "adq_solicitud_aprobar.html", data)
}

func (h *AdquisicionesHandler) SolicitudAprobarPost(w http.ResponseWriter, r *http.Request, id int) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulario inválido", http.StatusBadRequest)
		return
	}
	lineas := leerLineasSolicitud(r)
	visto := r.FormValue("vbjefe") == "S"
	if err := h.repo.AprobarSolicitud(id, visto, lineas); err != nil {
		h.pintarAprobacion(w, r, id, lineas, visto, mensajeOperacion(err))
		return
	}
	if visto {
		h.redirigir(w, r, "/adquisiciones/solicitudes/pendientes", "aprobado")
		return
	}
	h.redirigir(w, r, "/adquisiciones/solicitudes/pendientes", "rechazado")
}

func (h *AdquisicionesHandler) SolicitudEntregarPage(w http.ResponseWriter, r *http.Request, id int) {
	h.pintarEntrega(w, r, id, "")
}

func (h *AdquisicionesHandler) pintarEntrega(w http.ResponseWriter, r *http.Request, id int, errMsg string) {
	data := pageData(r)
	data["Title"] = "Entrega de materiales"
	data["Seccion"] = "solicitudes"
	sol, err := h.repo.Solicitud(id)
	if err != nil {
		data["Error"] = mensajeOperacion(err)
		h.render(w, "adq_solicitud_entregar.html", data)
		return
	}
	ctx, err := h.repo.ContextoEntrega(sol)
	if err != nil && errMsg == "" {
		errMsg = "No se pudo calcular el stock y el consumo."
	}
	data["Solicitud"] = sol
	data["Entregas"] = ctx
	if errMsg != "" {
		data["Error"] = errMsg
	}
	h.render(w, "adq_solicitud_entregar.html", data)
}

func (h *AdquisicionesHandler) SolicitudEntregarPost(w http.ResponseWriter, r *http.Request, id int) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulario inválido", http.StatusBadRequest)
		return
	}
	cantidades := map[int]int{}
	for clave, valores := range r.Form {
		if !strings.HasPrefix(clave, "entregada_") || len(valores) == 0 {
			continue
		}
		corr := entero(strings.TrimPrefix(clave, "entregada_"))
		if corr > 0 {
			cantidades[corr] = entero(valores[0])
		}
	}
	if err := h.repo.EntregarSolicitud(id, cantidades); err != nil {
		h.pintarEntrega(w, r, id, mensajeOperacion(err))
		return
	}
	h.redirigir(w, r, "/adquisiciones/solicitudes/cursadas", "entregado")
}

// ---------- Reportes ----------

func (h *AdquisicionesHandler) ReportesPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Reportes de adquisiciones"
	data["Seccion"] = "reportes"
	h.render(w, "adq_reportes.html", data)
}

func (h *AdquisicionesHandler) prepararReporte(r *http.Request, titulo string) (map[string]interface{}, models.FiltroReporte) {
	data := pageData(r)
	data["Title"] = titulo
	data["Seccion"] = "reportes"
	f := models.FiltroReporte{
		Depto:      entero(r.URL.Query().Get("depto")),
		Tipo:       entero(r.URL.Query().Get("tipo")),
		Material:   entero(r.URL.Query().Get("material")),
		Fecha1:     r.URL.Query().Get("fecha1"),
		Fecha2:     r.URL.Query().Get("fecha2"),
		FechaCorte: r.URL.Query().Get("fecha"),
		Anio:       entero(r.URL.Query().Get("anio")),
		Mes:        entero(r.URL.Query().Get("mes")),
		Umbral:     entero(r.URL.Query().Get("umbral")),
		UsaCritico: r.URL.Query().Get("opcion") != "D",
		Numero:     entero(r.URL.Query().Get("numero")),
	}
	if f.Anio == 0 {
		f.Anio = time.Now().Year()
	}
	if f.Mes == 0 {
		f.Mes = int(time.Now().Month())
	}
	deptos, _ := h.repo.Departamentos()
	tipos, _ := h.repo.DominiosPorTipo(models.CodTipoMaterial)
	materiales, _ := h.repo.MaterialesCatalogo(f.Tipo)
	data["Filtro"] = f
	data["Departamentos"] = deptos
	data["Tipos"] = tipos
	data["Materiales"] = materiales
	data["Meses"] = []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	data["AnioActual"] = time.Now().Year()
	return data, f
}

func (h *AdquisicionesHandler) cerrarReporte(w http.ResponseWriter, r *http.Request, data map[string]interface{}, columnas []string, filas [][]string, err error) {
	if err != nil {
		data["Error"] = mensajeOperacion(err)
		h.render(w, "adq_reporte.html", data)
		return
	}
	if r.URL.Query().Get("formato") == "csv" {
		escribirCSV(w, columnas, filas)
		return
	}
	q := r.URL.Query()
	q.Del("formato")
	data["Consulta"] = q.Encode()
	data["Columnas"] = columnas
	data["Filas"] = filas
	data["Consultado"] = true
	h.render(w, "adq_reporte.html", data)
}

func escribirCSV(w http.ResponseWriter, columnas []string, filas [][]string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=reporte.csv")
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	cw := csv.NewWriter(w)
	cw.Comma = ';'
	_ = cw.Write(columnas)
	for _, fila := range filas {
		_ = cw.Write(fila)
	}
	cw.Flush()
}

func (h *AdquisicionesHandler) ReporteConsumoPage(w http.ResponseWriter, r *http.Request) {
	data, f := h.prepararReporte(r, "Consumo valorizado")
	data["MuestraDepto"] = true
	data["MuestraTipo"] = true
	data["MuestraMaterial"] = true
	data["MuestraFechas"] = true
	data["Bajada"] = "Suma de cantidades y valores despachados, con los mismos filtros del reporte PHP."
	if r.URL.RawQuery == "" {
		h.render(w, "adq_reporte.html", data)
		return
	}
	filas, err := h.repo.ReporteConsumo(f)
	h.cerrarReporte(w, r, data, []string{"Departamento", "Tipo", "Material", "Cantidad", "Valor total"}, filas, err)
}

func (h *AdquisicionesHandler) ReporteStockPage(w http.ResponseWriter, r *http.Request) {
	data, f := h.prepararReporte(r, "Stock de productos")
	data["MuestraTipo"] = true
	data["MuestraMaterial"] = true
	data["Bajada"] = "Existencia actual: la última cantidad de cada material y valor unitario."
	filas, err := h.repo.ReporteStock(f)
	h.cerrarReporte(w, r, data, []string{"Tipo", "Material", "Stock", "Stock crítico"}, filas, err)
}

func (h *AdquisicionesHandler) ReporteStockMinimoPage(w http.ResponseWriter, r *http.Request) {
	data, f := h.prepararReporte(r, "Bajo stock mínimo")
	data["MuestraUmbral"] = true
	data["Bajada"] = "Materiales cuya existencia está bajo el stock crítico o bajo una cantidad indicada."
	if r.URL.Query().Get("opcion") == "" {
		h.render(w, "adq_reporte.html", data)
		return
	}
	filas, err := h.repo.ReporteStockMinimo(f)
	h.cerrarReporte(w, r, data, []string{"Tipo", "Material", "Stock", "Stock crítico"}, filas, err)
}

func (h *AdquisicionesHandler) ReporteStockHistoricoPage(w http.ResponseWriter, r *http.Request) {
	data, f := h.prepararReporte(r, "Stock histórico")
	data["MuestraTipo"] = true
	data["MuestraMaterial"] = true
	data["MuestraFechaCorte"] = true
	data["Bajada"] = "Existencia reconstruida con el último movimiento de cada lote hasta la fecha de corte."
	if f.FechaCorte == "" {
		h.render(w, "adq_reporte.html", data)
		return
	}
	filas, err := h.repo.ReporteStockHistorico(f)
	h.cerrarReporte(w, r, data, []string{"Tipo", "Material", "Stock"}, filas, err)
}

func (h *AdquisicionesHandler) ReporteEntregaPage(w http.ResponseWriter, r *http.Request) {
	data, f := h.prepararReporte(r, "Entrega promedio mensual")
	data["MuestraTipo"] = true
	data["MuestraMaterial"] = true
	data["MuestraAnioMes"] = true
	data["Bajada"] = "Total entregado desde enero hasta el mes anterior, y el promedio de esos meses."
	if r.URL.Query().Get("anio") == "" {
		h.render(w, "adq_reporte.html", data)
		return
	}
	filas, err := h.repo.ReporteEntregaPromedio(f)
	h.cerrarReporte(w, r, data, []string{"Tipo", "Material", "Total entregado", "Promedio mensual"}, filas, err)
}

func (h *AdquisicionesHandler) ReporteIngresoPage(w http.ResponseWriter, r *http.Request) {
	data, f := h.prepararReporte(r, "Ingreso de materiales")
	data["MuestraTipo"] = true
	data["MuestraMaterial"] = true
	data["MuestraFechas"] = true
	data["Bajada"] = "Líneas de facturas de proveedor ingresadas al stock."
	if r.URL.RawQuery == "" {
		h.render(w, "adq_reporte.html", data)
		return
	}
	filas, err := h.repo.ReporteIngresoMateriales(f)
	h.cerrarReporte(w, r, data, []string{"Proveedor", "Factura", "Orden", "Ingreso", "Tipo", "Material", "Cantidad", "Valor unitario"}, filas, err)
}

func (h *AdquisicionesHandler) ReporteDocumentoPage(w http.ResponseWriter, r *http.Request, tipo int, titulo string) {
	data := pageData(r)
	data["Title"] = titulo
	data["Seccion"] = "reportes"
	data["Documento"] = true
	data["TipoEsperado"] = tipo
	nro := entero(r.URL.Query().Get("numero"))
	data["Numero"] = nro
	if nro > 0 {
		orden, err := h.repo.Orden(nro)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				data["Error"] = "No existe una orden con el número " + strconv.Itoa(nro) + "."
			} else {
				data["Error"] = mensajeOperacion(err)
			}
		} else if orden.TipoOrden != tipo {
			data["Error"] = "La orden " + strconv.Itoa(nro) + " no es del tipo solicitado. Su tipo es " + orden.TipoNombre + "."
		} else {
			data["Orden"] = orden
		}
	}
	h.render(w, "adq_orden_ver.html", data)
}
