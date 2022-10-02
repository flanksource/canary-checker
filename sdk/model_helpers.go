package swagger

import "github.com/google/uuid"

func (c Component) GetUUID() *uuid.UUID {
	id, _ := uuid.Parse(c.Id)
	return &id
}
