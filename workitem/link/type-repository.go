package link

import (
	"fmt"
	"log"

	"golang.org/x/net/context"

	"github.com/almighty/almighty-core/app"
	"github.com/almighty/almighty-core/errors"
	"github.com/jinzhu/gorm"
	satoriuuid "github.com/satori/go.uuid"
)

// WorkItemLinkTypeRepository encapsulates storage & retrieval of work item link types
type WorkItemLinkTypeRepository interface {
	Create(ctx context.Context, name string, description *string, sourceTypeName, targetTypeName, forwardName, reverseName, topology string, linkCategory satoriuuid.UUID) (*app.WorkItemLinkTypeSingle, error)
	Load(ctx context.Context, ID string) (*app.WorkItemLinkTypeSingle, error)
	List(ctx context.Context) (*app.WorkItemLinkTypeList, error)
	Delete(ctx context.Context, ID string) error
	Save(ctx context.Context, linkCat app.WorkItemLinkTypeSingle) (*app.WorkItemLinkTypeSingle, error)
}

// NewWorkItemLinkTypeRepository creates a work item link type repository based on gorm
func NewWorkItemLinkTypeRepository(db *gorm.DB) *GormWorkItemLinkTypeRepository {
	return &GormWorkItemLinkTypeRepository{db}
}

// GormWorkItemLinkTypeRepository implements WorkItemLinkTypeRepository using gorm
type GormWorkItemLinkTypeRepository struct {
	db *gorm.DB
}

// Create creates a new work item link type in the repository.
// Returns BadParameterError, ConversionError or InternalError
func (r *GormWorkItemLinkTypeRepository) Create(ctx context.Context, name string, description *string, sourceTypeName, targetTypeName, forwardName, reverseName, topology string, linkCategoryID satoriuuid.UUID) (*app.WorkItemLinkTypeSingle, error) {
	linkType := &WorkItemLinkType{
		Name:           name,
		Description:    description,
		SourceTypeName: sourceTypeName,
		TargetTypeName: targetTypeName,
		ForwardName:    forwardName,
		ReverseName:    reverseName,
		Topology:       topology,
		LinkCategoryID: linkCategoryID,
	}
	if err := linkType.CheckValidForCreation(); err != nil {
		return nil, err
	}

	// Check link category exists
	linkCategory := WorkItemLinkCategory{}
	db := r.db.Where("id=?", linkType.LinkCategoryID).Find(&linkCategory)
	if db.RecordNotFound() {
		return nil, errors.NewBadParameterError("work item link category", linkType.LinkCategoryID)
	}
	if db.Error != nil {
		return nil, errors.NewInternalError(fmt.Sprintf("Failed to find work item link category: %s", db.Error.Error()))
	}
	db = r.db.Create(linkType)
	if db.Error != nil {
		return nil, errors.NewInternalError(db.Error.Error())
	}
	// Convert the created link type entry into a JSONAPI response
	result := ConvertLinkTypeFromModel(*linkType)
	return &result, nil
}

// Load returns the work item link type for the given ID.
// Returns NotFoundError, ConversionError or InternalError
func (r *GormWorkItemLinkTypeRepository) Load(ctx context.Context, ID string) (*app.WorkItemLinkTypeSingle, error) {
	id, err := satoriuuid.FromString(ID)
	if err != nil {
		// treat as not found: clients don't know it must be a UUID
		return nil, errors.NewNotFoundError("work item link type", ID)
	}
	log.Printf("loading work item link type %s", id.String())
	res := WorkItemLinkType{}
	db := r.db.Model(&res).Where("id=?", ID).First(&res)
	if db.RecordNotFound() {
		log.Printf("not found work item link type, res=%v", res)
		return nil, errors.NewNotFoundError("work item link type", id.String())
	}
	if db.Error != nil {
		return nil, errors.NewInternalError(db.Error.Error())
	}
	// Convert the created link type entry into a JSONAPI response
	result := ConvertLinkTypeFromModel(res)

	return &result, nil
}

// LoadTypeFromDB return work item link type for the given name in the correct link category
// NOTE: Two link types can coexist with different categoryIDs.
func (r *GormWorkItemLinkTypeRepository) LoadTypeFromDBByNameAndCategory(name string, categoryId satoriuuid.UUID) (*WorkItemLinkType, error) {
	log.Printf("loading work item link type %s with category ID %s", name, categoryId.String())
	res := WorkItemLinkType{}
	db := r.db.Model(&res).Where("name=? AND link_category_id=?", name, categoryId.String()).First(&res)
	if db.RecordNotFound() {
		log.Printf("not found, res=%v", res)
		return nil, errors.NewNotFoundError("work item link type", name)
	}
	if db.Error != nil {
		return nil, errors.NewInternalError(db.Error.Error())
	}
	return &res, nil
}

// LoadTypeFromDB return work item link type for the given ID
func (r *GormWorkItemLinkTypeRepository) LoadTypeFromDBByID(ID satoriuuid.UUID) (*WorkItemLinkType, error) {
	log.Printf("loading work item link type with ID %s", ID)
	res := WorkItemLinkType{}
	db := r.db.Model(&res).Where("ID=?", ID.String()).First(&res)
	if db.RecordNotFound() {
		log.Printf("not found, res=%v", res)
		return nil, errors.NewNotFoundError("work item link type", ID.String())
	}
	if db.Error != nil {
		return nil, errors.NewInternalError(db.Error.Error())
	}
	return &res, nil
}

// List returns all work item link types
// TODO: Handle pagination
func (r *GormWorkItemLinkTypeRepository) List(ctx context.Context) (*app.WorkItemLinkTypeList, error) {
	// We don't have any where clause or paging at the moment.
	var rows []WorkItemLinkType
	db := r.db.Find(&rows)
	if db.Error != nil {
		return nil, db.Error
	}
	res := app.WorkItemLinkTypeList{}
	res.Data = make([]*app.WorkItemLinkTypeData, len(rows))
	for index, value := range rows {
		linkType := ConvertLinkTypeFromModel(value)
		res.Data[index] = linkType.Data
	}
	// TODO: When adding pagination, this must not be len(rows) but
	// the overall total number of elements from all pages.
	res.Meta = &app.WorkItemLinkTypeListMeta{
		TotalCount: len(rows),
	}
	return &res, nil
}

// Delete deletes the work item link type with the given id
// returns NotFoundError or InternalError
func (r *GormWorkItemLinkTypeRepository) Delete(ctx context.Context, ID string) error {
	id, err := satoriuuid.FromString(ID)
	if err != nil {
		// treat as not found: clients don't know it must be a UUID
		return errors.NewNotFoundError("work item link type", ID)
	}
	var cat = WorkItemLinkType{
		ID: id,
	}
	log.Printf("work item link type to delete %v\n", cat)
	db := r.db.Delete(&cat)
	if db.Error != nil {
		return errors.NewInternalError(db.Error.Error())
	}
	if db.RowsAffected == 0 {
		return errors.NewNotFoundError("work item link type", id.String())
	}
	return nil
}

// Save updates the given work item link type in storage. Version must be the same as the one int the stored version.
// returns NotFoundError, VersionConflictError, ConversionError or InternalError
func (r *GormWorkItemLinkTypeRepository) Save(ctx context.Context, lt app.WorkItemLinkTypeSingle) (*app.WorkItemLinkTypeSingle, error) {
	res := WorkItemLinkType{}
	if lt.Data.ID == nil {
		return nil, errors.NewBadParameterError("work item link type", nil)
	}
	db := r.db.Model(&res).Where("id=?", *lt.Data.ID).First(&res)
	if db.RecordNotFound() {
		log.Printf("work item link type not found, res=%v", res)
		return nil, errors.NewNotFoundError("work item link type", *lt.Data.ID)
	}
	if db.Error != nil {
		log.Print(db.Error.Error())
		return nil, errors.NewInternalError(db.Error.Error())
	}
	if lt.Data.Attributes.Version == nil || res.Version != *lt.Data.Attributes.Version {
		return nil, errors.NewVersionConflictError("version conflict")
	}
	if err := ConvertLinkTypeToModel(lt, &res); err != nil {
		return nil, err
	}
	res.Version = res.Version + 1
	db = db.Save(&res)
	if db.Error != nil {
		log.Print(db.Error.Error())
		return nil, errors.NewInternalError(db.Error.Error())
	}
	log.Printf("updated work item link type to %v\n", res)
	result := ConvertLinkTypeFromModel(res)
	return &result, nil
}
