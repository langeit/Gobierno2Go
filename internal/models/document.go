package models

// Document representa un documento de correspondencia (tabla cor_docto).
type Document struct {
	Barra        string `json:"barra"`
	Tipo         string `json:"tipo"`
	TipoDesc     string `json:"tipo_desc,omitempty"`
	Origen       string `json:"origen"`
	OrigenDesc   string `json:"origen_desc,omitempty"`
	Accion       string `json:"accion"`
	AccionDesc   string `json:"accion_desc,omitempty"`
	Referencia   string `json:"referencia"`
	FechCreacion string `json:"fechcreacion"`
	Numero       int    `json:"numero"`
	Fecha        string `json:"fecha,omitempty"`
	Grupo        int    `json:"grupo,omitempty"`
	Origen2      string `json:"origen2,omitempty"`
	Notas        string `json:"notas,omitempty"`
}

// CreateDocumentInput agrupa los datos necesarios para el ingreso de un documento.
type CreateDocumentInput struct {
	Tipo       string `json:"tipo"`
	Origen     string `json:"origen"`
	Accion     string `json:"accion"`
	Referencia string `json:"referencia"`
	Origen2    string `json:"origen2"`
	Notas      string `json:"notas"`
	Numero     int    `json:"numero"`
	Fecha      string `json:"fecha"` // fecha del documento (YYYY-MM-DD), opcional
	// Destino inmediato (opcional). Sólo uno de los dos debe venir informado.
	EnviarAUsuario string `json:"enviar_a_usuario"`
	EnviarAEtapa   int    `json:"enviar_a_etapa"`
}

// PendingDocument es un documento en tránsito hacia el usuario o centro actual.
type PendingDocument struct {
	Barra        string `json:"barra"`
	TipoDesc     string `json:"tipo_desc"`
	Referencia   string `json:"referencia"`
	FechCreacion string `json:"fechcreacion"`
	Origen       string `json:"origen"`
}

// HeldDocument es un documento que el usuario (o su centro) tiene en su poder.
type HeldDocument struct {
	Barra      string `json:"barra"`
	TipoDesc   string `json:"tipo_desc"`
	Referencia string `json:"referencia"`
	Actual     string `json:"actual"`
}

// HistoriaItem es un movimiento de la bitácora del documento.
type HistoriaItem struct {
	Correlativo int    `json:"correlativo"`
	Fecha       string `json:"fecha"`
	Movimiento  string `json:"movimiento"`
	Usuario     string `json:"usuario"`
	Actual      string `json:"actual"`
	Comodin     string `json:"comodin"`
}

// Ficha agrupa el documento, su ubicación actual y su historial.
type Ficha struct {
	Document     Document       `json:"document"`
	Actual       string         `json:"actual"`
	Etapa        string         `json:"etapa"`
	Destino      string         `json:"destino"`
	EtapaDestino string         `json:"etapa_destino"`
	Archivado    bool           `json:"archivado"`
	Historia     []HistoriaItem `json:"historia"`
}

// CatalogItem es una entrada de catálogo (tipo, origen, acción, etapa).
type CatalogItem struct {
	Codigo      string `json:"codigo"`
	Descripcion string `json:"descripcion"`
}

// UserOption es un usuario seleccionable como destino.
type UserOption struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
}

// DestinationOption es una opción de envío (usuario o etapa) lista para un <select>.
type DestinationOption struct {
	Value string
	Label string
	Group string // "Usuario (mi centro)" o "Etapa"
}

// DocumentStats resume las métricas del módulo.
type DocumentStats struct {
	TotalDocuments int `json:"total_documents"`
	Pendientes     int `json:"pendientes"`
	Archivados     int `json:"archivados"`
	Recepcionados  int `json:"recepcionados"`
}

// CountItem es una fila de agrupación (por tipo, por etapa, etc.).
type CountItem struct {
	Descripcion string `json:"descripcion"`
	Total       int    `json:"total"`
}

// ArchivedDocument es una fila del listado de documentos archivados.
type ArchivedDocument struct {
	Barra      string `json:"barra"`
	TipoDesc   string `json:"tipo_desc"`
	Referencia string `json:"referencia"`
	Fecha      string `json:"fecha"`
	Carpeta    string `json:"carpeta"`
}
