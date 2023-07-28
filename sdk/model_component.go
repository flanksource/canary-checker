/*
 * Canary Checker API
 *
 * No description provided (generated by Swagger Codegen https://github.com/swagger-api/swagger-codegen)
 *
 * API version: 1..1
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package swagger

type Component struct {
	Checks     []Check      `json:"checks,omitempty"`
	Components *[]Component `json:"components,omitempty"`
	Configs    []Config     `json:"configs,omitempty"`
	CreatedAt  string       `json:"created_at,omitempty"`
	// nolint
	ExternalId string `json:"external_id,omitempty"`
	Icon       string `json:"icon,omitempty"`
	// nolint
	Id     string       `json:"id,omitempty"`
	Labels *interface{} `json:"labels,omitempty"`
	// The lifecycle state of the component e.g. production, staging, dev, etc.
	Lifecycle string `json:"lifecycle,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Order     int32  `json:"order,omitempty"`
	Owner     string `json:"owner,omitempty"`
	// nolint
	ParentId     string     `json:"parent_id,omitempty"`
	Path         string     `json:"path,omitempty"`
	Properties   []Property `json:"properties,omitempty"`
	Schedule     string     `json:"schedule,omitempty"`
	Status       string     `json:"status,omitempty"`
	StatusReason string     `json:"status_reason,omitempty"`
	Summary      *Summary   `json:"summary,omitempty"`
	// nolint
	TopologyID   string `json:"topology_id,omitempty"`
	Text         string `json:"text,omitempty"`
	Tooltip      string `json:"tooltip,omitempty"`
	TopologyType string `json:"topology_type,omitempty"`
	// The type of component, e.g. service, API, website, library, database, etc.
	Type_     string `json:"type,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}
