package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"Gobierno2Go/internal/config"
	"Gobierno2Go/internal/handlers"
	"Gobierno2Go/internal/repository"
	"Gobierno2Go/internal/session"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	db, err := config.ConnectDB()
	if err != nil {
		log.Fatalf("Error conectando a la base de datos: %v", err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	docRepo := repository.NewDocumentRepository(db)
	catalogRepo := repository.NewCatalogRepository(db)
	formularioRepo := repository.NewFormularioRepository(db)
	inventarioRepo := repository.NewInventarioRepository(db)
	accesoRepo := repository.NewAccesoRepository(db)
	if err := accesoRepo.EnsureSchema(); err != nil {
		log.Fatalf("Error preparando perfiles de acceso: %v", err)
	}
	adquisicionesRepo := repository.NewAdquisicionesRepository(db)
	if err := adquisicionesRepo.EnsureSchema(); err != nil {
		log.Fatalf("Error preparando mantenedores de adquisiciones: %v", err)
	}
	if err := userRepo.MigrarContrasenas(); err != nil {
		log.Fatalf("Error migrando contraseñas a bcrypt: %v", err)
	}

	sessions := session.NewManager()

	loginHandler := handlers.NewLoginHandler(userRepo, sessions)
	pageHandler := handlers.NewPageHandler("internal/templates")
	correspondenciaHandler := handlers.NewCorrespondenciaHandler(docRepo, catalogRepo, userRepo, "internal/templates")
	formularioHandler := handlers.NewFormularioHandler(formularioRepo, "internal/templates")
	inventarioHandler := handlers.NewInventarioHandler(inventarioRepo, "internal/templates")
	accesoHandler := handlers.NewAccesoHandler(accesoRepo, "internal/templates")
	adquisicionesHandler := handlers.NewAdquisicionesHandler(adquisicionesRepo, "internal/templates")

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(sessions.Middleware)

	// Rutas públicas
	r.Get("/", pageHandler.LoginPage)
	r.Post("/api/v1/login", loginHandler.Login)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","message":"Gobierno2Go API is running"}`))
	})
	r.Get("/api/v1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Welcome to Gobierno2Go API v1"}`))
	})

	// Rutas protegidas
	r.Group(func(r chi.Router) {
		r.Use(session.RequireLogin)
		r.Use(session.Enrich(accesoRepo.ModulosDeUsuario))

		r.Get("/logout", loginHandler.Logout)
		r.Get("/dashboard", pageHandler.DashboardPage)

		r.Group(func(r chi.Router) {
			r.Use(session.RequireModule("correspondencia"))
			r.Get("/correspondencia", correspondenciaHandler.MenuPage)
			r.Get("/correspondencia/ingreso", correspondenciaHandler.IngresoPage)
			r.Post("/correspondencia/ingreso", correspondenciaHandler.IngresoPost)
			r.Get("/correspondencia/recepcion", correspondenciaHandler.RecepcionPage)
			r.Post("/correspondencia/recepcion", correspondenciaHandler.RecepcionPost)
			r.Get("/correspondencia/despacho", correspondenciaHandler.DespachoPage)
			r.Post("/correspondencia/despacho", correspondenciaHandler.DespachoPost)
			r.Get("/correspondencia/archivo", correspondenciaHandler.ArchivoPage)
			r.Post("/correspondencia/archivo", correspondenciaHandler.ArchivoPost)
			r.Get("/correspondencia/consulta", correspondenciaHandler.ConsultaPage)
			r.Get("/correspondencia/ficha", correspondenciaHandler.FichaPage)
			r.Get("/correspondencia/estadisticas", correspondenciaHandler.EstadisticasPage)

			r.Route("/api/v1/correspondencia", func(r chi.Router) {
				r.Get("/documents", correspondenciaHandler.ApiGetDocuments)
				r.Post("/documents", correspondenciaHandler.ApiCreateDocument)
				r.Get("/stats", correspondenciaHandler.ApiGetStats)
				r.Get("/pending", correspondenciaHandler.ApiPending)
				r.Post("/receive", correspondenciaHandler.ApiReceive)
				r.Post("/dispatch", correspondenciaHandler.ApiDispatch)
				r.Get("/ficha", correspondenciaHandler.ApiGetFicha)
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(session.RequireModule("formularios"))
			r.Get("/formularios", formularioHandler.MenuPage)
			r.Get("/formularios/feriado", formularioHandler.FeriadoPage)
			r.Post("/formularios/feriado", formularioHandler.FeriadoPost)
			r.Get("/formularios/administrativo", formularioHandler.AdministrativoPage)
			r.Post("/formularios/administrativo", formularioHandler.AdministrativoPost)
			r.Get("/formularios/personal/feriado", formularioHandler.PersonalFeriadoPage)
			r.Post("/formularios/personal/feriado", formularioHandler.PersonalFeriadoPost)
			r.Get("/formularios/personal/administrativo", formularioHandler.PersonalAdministrativoPage)
			r.Post("/formularios/personal/administrativo", formularioHandler.PersonalAdministrativoPost)
			r.Get("/formularios/listado", formularioHandler.ListadoPage)
			r.Post("/formularios/eliminar", formularioHandler.EliminarPost)
			r.Get("/formularios/aprobaciones", formularioHandler.AprobacionesPage)
			r.Post("/formularios/aprobaciones", formularioHandler.AprobacionesPost)
			r.Get("/formularios/acumulacion", formularioHandler.AcumulacionPage)
			r.Post("/formularios/acumulacion", formularioHandler.AcumulacionPost)
			r.Post("/formularios/acumulacion/aprobar", formularioHandler.AcumulacionAprobarPost)
			r.Get("/formularios/admin", formularioHandler.AdminPage)
			r.Get("/formularios/admin/asignacion", formularioHandler.AsignacionPage)
			r.Post("/formularios/admin/asignacion", formularioHandler.AsignacionPost)
			r.Get("/formularios/admin/jefes", formularioHandler.JefesPage)
			r.Post("/formularios/admin/jefes", formularioHandler.JefesPost)
			r.Get("/formularios/admin/trabajadores", formularioHandler.TrabajadoresAdminPage)
			r.Get("/formularios/admin/trabajadores/nuevo", formularioHandler.TrabajadorNuevoPage)
			r.Post("/formularios/admin/trabajadores/nuevo", formularioHandler.TrabajadorNuevoPost)
			r.Get("/formularios/admin/trabajadores/{id}", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				formularioHandler.TrabajadorEditarPage(w, r, id)
			})
			r.Post("/formularios/admin/trabajadores/{id}", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				formularioHandler.TrabajadorEditarPost(w, r, id)
			})
			r.Post("/formularios/admin/trabajadores/{id}/eliminar", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				formularioHandler.TrabajadorEliminarPost(w, r, id)
			})
			r.Get("/formularios/admin/departamentos", formularioHandler.DepartamentosAdminPage)
			r.Get("/formularios/admin/departamentos/nuevo", formularioHandler.DepartamentoNuevoPage)
			r.Post("/formularios/admin/departamentos/nuevo", formularioHandler.DepartamentoNuevoPost)
			r.Get("/formularios/admin/departamentos/{id}", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				formularioHandler.DepartamentoEditarPage(w, r, id)
			})
			r.Post("/formularios/admin/departamentos/{id}", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				formularioHandler.DepartamentoEditarPost(w, r, id)
			})
			r.Post("/formularios/admin/departamentos/{id}/eliminar", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				formularioHandler.DepartamentoEliminarPost(w, r, id)
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(session.RequireModule("inventario"))
			r.Get("/inventario", inventarioHandler.MenuPage)
			r.Get("/inventario/bienes", inventarioHandler.BienesPage)
			r.Get("/inventario/ingreso", inventarioHandler.IngresoPage)
			r.Post("/inventario/ingreso", inventarioHandler.IngresoPost)
			r.Get("/inventario/ubicacion", inventarioHandler.UbicacionPage)
			r.Post("/inventario/ubicacion", inventarioHandler.UbicacionPost)
			r.Get("/inventario/estado", inventarioHandler.EstadoPage)
			r.Post("/inventario/estado", inventarioHandler.EstadoPost)
			r.Get("/inventario/cuenta", inventarioHandler.CuentaPage)
			r.Post("/inventario/cuenta", inventarioHandler.CuentaPost)
			r.Get("/inventario/responsable", inventarioHandler.ResponsablePage)
			r.Post("/inventario/responsable", inventarioHandler.ResponsablePost)

			r.Get("/inventario/solicitudes", inventarioHandler.SolicitudesPage)
			r.Get("/inventario/solicitudes/nueva", inventarioHandler.SolicitudNuevaPage)
			r.Post("/inventario/solicitudes/nueva", inventarioHandler.SolicitudNuevaPost)
			r.Get("/inventario/solicitudes/{id}", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				inventarioHandler.SolicitudDetallePage(w, r, id)
			})
			r.Post("/inventario/solicitudes/{id}/resolver", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				inventarioHandler.SolicitudResolverPost(w, r, id)
			})

			r.Get("/inventario/porcentaje", inventarioHandler.PorcentajePage)
			r.Post("/inventario/porcentaje", inventarioHandler.PorcentajePost)
			r.Get("/inventario/depreciacion", inventarioHandler.DepreciacionPage)
			r.Post("/inventario/depreciacion", inventarioHandler.DepreciacionPost)

			r.Get("/inventario/reportes", inventarioHandler.ReportesPage)
			r.Get("/inventario/reportes/responsable", inventarioHandler.ReporteResponsablePage)
			r.Get("/inventario/reportes/ubicacion", inventarioHandler.ReporteUbicacionPage)
			r.Get("/inventario/reportes/anio", inventarioHandler.ReporteAnioPage)
			r.Get("/inventario/reportes/movimientos", inventarioHandler.ReporteMovimientosPage)
			r.Get("/inventario/reportes/depreciacion", inventarioHandler.ReporteDepreciacionPage)
			r.Get("/inventario/reportes/depreciacion/detalle", inventarioHandler.ReporteDepreciacionDetallePage)

			r.Get("/inventario/admin", inventarioHandler.AdminPage)
			r.Post("/inventario/admin/tipo-bien", inventarioHandler.AdminTipoBienPost)
			r.Post("/inventario/admin/zona", inventarioHandler.AdminZonaPost)
		})

		r.Group(func(r chi.Router) {
			r.Use(session.RequireModule("adquisiciones"))
			r.Get("/adquisiciones", adquisicionesHandler.MenuPage)
			r.Get("/adquisiciones/mantenedores", adquisicionesHandler.MantenedoresPage)

			r.Get("/adquisiciones/mantenedores/tipos-dominio", adquisicionesHandler.TiposPage)
			r.Get("/adquisiciones/mantenedores/tipos-dominio/nuevo", adquisicionesHandler.TipoNuevoPage)
			r.Post("/adquisiciones/mantenedores/tipos-dominio/nuevo", adquisicionesHandler.TipoNuevoPost)
			r.Get("/adquisiciones/mantenedores/tipos-dominio/{id}", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.TipoVerPage(w, r, id)
			})
			r.Get("/adquisiciones/mantenedores/tipos-dominio/{id}/editar", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.TipoEditarPage(w, r, id)
			})
			r.Post("/adquisiciones/mantenedores/tipos-dominio/{id}", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.TipoEditarPost(w, r, id)
			})
			r.Post("/adquisiciones/mantenedores/tipos-dominio/{id}/eliminar", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.TipoEliminarPost(w, r, id)
			})

			r.Get("/adquisiciones/mantenedores/dominios", adquisicionesHandler.DominiosPage)
			r.Get("/adquisiciones/mantenedores/dominios/nuevo", adquisicionesHandler.DominioNuevoPage)
			r.Post("/adquisiciones/mantenedores/dominios/nuevo", adquisicionesHandler.DominioNuevoPost)
			r.Get("/adquisiciones/mantenedores/dominios/{id}", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.DominioVerPage(w, r, id)
			})
			r.Get("/adquisiciones/mantenedores/dominios/{id}/editar", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.DominioEditarPage(w, r, id)
			})
			r.Post("/adquisiciones/mantenedores/dominios/{id}", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.DominioEditarPost(w, r, id)
			})
			r.Post("/adquisiciones/mantenedores/dominios/{id}/eliminar", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.DominioEliminarPost(w, r, id)
			})

			r.Get("/adquisiciones/mantenedores/materiales", adquisicionesHandler.MaterialesPage)
			r.Get("/adquisiciones/mantenedores/materiales/nuevo", adquisicionesHandler.MaterialNuevoPage)
			r.Post("/adquisiciones/mantenedores/materiales/nuevo", adquisicionesHandler.MaterialNuevoPost)
			r.Get("/adquisiciones/mantenedores/materiales/{id}", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.MaterialVerPage(w, r, id)
			})
			r.Get("/adquisiciones/mantenedores/materiales/{id}/editar", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.MaterialEditarPage(w, r, id)
			})
			r.Post("/adquisiciones/mantenedores/materiales/{id}", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.MaterialEditarPost(w, r, id)
			})
			r.Post("/adquisiciones/mantenedores/materiales/{id}/eliminar", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.MaterialEliminarPost(w, r, id)
			})

			r.Get("/adquisiciones/mantenedores/proveedores", adquisicionesHandler.ProveedoresPage)
			r.Get("/adquisiciones/mantenedores/proveedores/nuevo", adquisicionesHandler.ProveedorNuevoPage)
			r.Post("/adquisiciones/mantenedores/proveedores/nuevo", adquisicionesHandler.ProveedorNuevoPost)
			r.Get("/adquisiciones/mantenedores/proveedores/{rut}", func(w http.ResponseWriter, r *http.Request) {
				adquisicionesHandler.ProveedorVerPage(w, r, chi.URLParam(r, "rut"))
			})
			r.Get("/adquisiciones/mantenedores/proveedores/{rut}/editar", func(w http.ResponseWriter, r *http.Request) {
				adquisicionesHandler.ProveedorEditarPage(w, r, chi.URLParam(r, "rut"))
			})
			r.Post("/adquisiciones/mantenedores/proveedores/{rut}", func(w http.ResponseWriter, r *http.Request) {
				adquisicionesHandler.ProveedorEditarPost(w, r, chi.URLParam(r, "rut"))
			})
			r.Post("/adquisiciones/mantenedores/proveedores/{rut}/eliminar", func(w http.ResponseWriter, r *http.Request) {
				adquisicionesHandler.ProveedorEliminarPost(w, r, chi.URLParam(r, "rut"))
			})

			r.Get("/adquisiciones/mantenedores/asignaciones", adquisicionesHandler.AsignacionesPage)
			r.Get("/adquisiciones/mantenedores/asignaciones/nuevo", adquisicionesHandler.AsignacionNuevaPage)
			r.Post("/adquisiciones/mantenedores/asignaciones/nuevo", adquisicionesHandler.AsignacionNuevaPost)
			r.Get("/adquisiciones/mantenedores/asignaciones/{id}", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.AsignacionVerPage(w, r, id)
			})
			r.Get("/adquisiciones/mantenedores/asignaciones/{id}/editar", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.AsignacionEditarPage(w, r, id)
			})
			r.Post("/adquisiciones/mantenedores/asignaciones/{id}", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.AsignacionEditarPost(w, r, id)
			})
			r.Post("/adquisiciones/mantenedores/asignaciones/{id}/eliminar", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.AsignacionEliminarPost(w, r, id)
			})

			r.Get("/adquisiciones/ordenes", adquisicionesHandler.OrdenesPage)
			r.Get("/adquisiciones/ordenes/nueva", adquisicionesHandler.OrdenNuevaPage)
			r.Post("/adquisiciones/ordenes/nueva", adquisicionesHandler.OrdenNuevaPost)
			r.Get("/adquisiciones/ordenes/{nro}", func(w http.ResponseWriter, r *http.Request) {
				nro, _ := strconv.Atoi(chi.URLParam(r, "nro"))
				adquisicionesHandler.OrdenVerPage(w, r, nro)
			})
			r.Get("/adquisiciones/ordenes/{nro}/editar", func(w http.ResponseWriter, r *http.Request) {
				nro, _ := strconv.Atoi(chi.URLParam(r, "nro"))
				adquisicionesHandler.OrdenEditarPage(w, r, nro)
			})
			r.Post("/adquisiciones/ordenes/{nro}", func(w http.ResponseWriter, r *http.Request) {
				nro, _ := strconv.Atoi(chi.URLParam(r, "nro"))
				adquisicionesHandler.OrdenEditarPost(w, r, nro)
			})
			r.Post("/adquisiciones/ordenes/{nro}/eliminar", func(w http.ResponseWriter, r *http.Request) {
				nro, _ := strconv.Atoi(chi.URLParam(r, "nro"))
				adquisicionesHandler.OrdenEliminarPost(w, r, nro)
			})

			r.Get("/adquisiciones/facturas", adquisicionesHandler.FacturasPage)
			r.Get("/adquisiciones/facturas/nueva", adquisicionesHandler.FacturaNuevaPage)
			r.Post("/adquisiciones/facturas/nueva", adquisicionesHandler.FacturaNuevaPost)
			r.Get("/adquisiciones/facturas/{id}", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.FacturaVerPage(w, r, id)
			})

			r.Get("/adquisiciones/solicitudes", adquisicionesHandler.SolicitudesPage)
			r.Get("/adquisiciones/solicitudes/nueva", adquisicionesHandler.SolicitudNuevaPage)
			r.Post("/adquisiciones/solicitudes/nueva", adquisicionesHandler.SolicitudNuevaPost)
			r.Get("/adquisiciones/solicitudes/por-aprobar", func(w http.ResponseWriter, r *http.Request) {
				adquisicionesHandler.SolicitudesEstadoPage(w, r, 25, "Solicitudes por aprobar", "Pedidos que esperan el visto bueno.")
			})
			r.Get("/adquisiciones/solicitudes/pendientes", func(w http.ResponseWriter, r *http.Request) {
				adquisicionesHandler.SolicitudesEstadoPage(w, r, 5, "Solicitudes pendientes de entrega", "Incluye las aprobadas y las rechazadas. Solo las que tienen visto bueno se pueden entregar.")
			})
			r.Get("/adquisiciones/solicitudes/cursadas", func(w http.ResponseWriter, r *http.Request) {
				adquisicionesHandler.SolicitudesEstadoPage(w, r, 6, "Solicitudes cursadas", "Entregas ya registradas.")
			})
			r.Get("/adquisiciones/solicitudes/{id}", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.SolicitudVerPage(w, r, id)
			})
			r.Get("/adquisiciones/solicitudes/{id}/aprobar", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.SolicitudAprobarPage(w, r, id)
			})
			r.Post("/adquisiciones/solicitudes/{id}/aprobar", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.SolicitudAprobarPost(w, r, id)
			})
			r.Get("/adquisiciones/solicitudes/{id}/entregar", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.SolicitudEntregarPage(w, r, id)
			})
			r.Post("/adquisiciones/solicitudes/{id}/entregar", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				adquisicionesHandler.SolicitudEntregarPost(w, r, id)
			})

			r.Get("/adquisiciones/reportes", adquisicionesHandler.ReportesPage)
			r.Get("/adquisiciones/reportes/consumo", adquisicionesHandler.ReporteConsumoPage)
			r.Get("/adquisiciones/reportes/stock-minimo", adquisicionesHandler.ReporteStockMinimoPage)
			r.Get("/adquisiciones/reportes/stock", adquisicionesHandler.ReporteStockPage)
			r.Get("/adquisiciones/reportes/stock-historico", adquisicionesHandler.ReporteStockHistoricoPage)
			r.Get("/adquisiciones/reportes/entrega-promedio", adquisicionesHandler.ReporteEntregaPage)
			r.Get("/adquisiciones/reportes/orden-pedido", func(w http.ResponseWriter, r *http.Request) {
				adquisicionesHandler.ReporteDocumentoPage(w, r, 10, "Orden de pedido")
			})
			r.Get("/adquisiciones/reportes/resolucion-compra", func(w http.ResponseWriter, r *http.Request) {
				adquisicionesHandler.ReporteDocumentoPage(w, r, 13, "Resolución administrativa de compra")
			})
			r.Get("/adquisiciones/reportes/resolucion-exenta", func(w http.ResponseWriter, r *http.Request) {
				adquisicionesHandler.ReporteDocumentoPage(w, r, 12, "Resolución administrativa exenta")
			})
			r.Get("/adquisiciones/reportes/ingreso-materiales", adquisicionesHandler.ReporteIngresoPage)
		})

		r.Group(func(r chi.Router) {
			r.Use(session.RequireModule("administracion"))
			r.Get("/admin", accesoHandler.MenuPage)
			r.Get("/admin/perfiles", accesoHandler.PerfilesPage)
			r.Post("/admin/perfiles", accesoHandler.PerfilNuevoPost)
			r.Get("/admin/perfiles/{id}", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				accesoHandler.PerfilPage(w, r, id)
			})
			r.Post("/admin/perfiles/{id}", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				accesoHandler.PerfilPost(w, r, id)
			})
			r.Post("/admin/perfiles/{id}/eliminar", func(w http.ResponseWriter, r *http.Request) {
				id, _ := strconv.Atoi(chi.URLParam(r, "id"))
				accesoHandler.PerfilEliminarPost(w, r, id)
			})
			r.Get("/admin/usuarios", accesoHandler.UsuariosPage)
			r.Post("/admin/usuarios", accesoHandler.UsuarioNuevoPost)
			r.Post("/admin/usuarios/{id}", func(w http.ResponseWriter, r *http.Request) {
				accesoHandler.UsuarioPost(w, r, chi.URLParam(r, "id"))
			})
		})
	})

	log.Printf("Starting Gobierno2Go server on port %s...", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
