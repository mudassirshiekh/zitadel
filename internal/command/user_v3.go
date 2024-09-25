package command

import (
	"context"
	"encoding/json"

	"github.com/zitadel/zitadel/internal/domain"
	"github.com/zitadel/zitadel/internal/zerrors"
)

type CreateSchemaUser struct {
	ResourceOwner string
	ID            string

	SchemaID string
	Data     json.RawMessage

	Email           *Email
	ReturnCodeEmail *string
	Phone           *Phone
	ReturnCodePhone *string
}

func (s *CreateSchemaUser) Valid() (err error) {
	if s.ResourceOwner == "" {
		return zerrors.ThrowInvalidArgument(nil, "COMMAND-urEJKa1tJM", "Errors.ResourceOwnerMissing")
	}
	if s.SchemaID == "" {
		return zerrors.ThrowInvalidArgument(nil, "COMMAND-TFo06JgnF2", "Errors.UserSchema.ID.Missing")
	}

	if s.Email != nil && s.Email.Address != "" {
		if err := s.Email.Validate(); err != nil {
			return err
		}
	}

	if s.Phone != nil && s.Phone.Number != "" {
		if s.Phone.Number, err = s.Phone.Number.Normalize(); err != nil {
			return err
		}
	}

	return nil
}

func (c *Commands) CreateSchemaUser(ctx context.Context, user *CreateSchemaUser) (_ *domain.ObjectDetails, err error) {
	if err := user.Valid(); err != nil {
		return nil, err
	}

	if user.ID == "" {
		user.ID, err = c.idGenerator.Next()
		if err != nil {
			return nil, err
		}
	}

	writeModel, err := c.getSchemaUserWMForState(ctx, user.ResourceOwner, user.ID)
	if err != nil {
		return nil, err
	}

	schemaWriteModel, err := existingSchema(ctx, c, "", user.SchemaID)
	if err != nil {
		return nil, err
	}

	events, codeEmail, codePhone, err := writeModel.NewCreate(ctx,
		schemaWriteModel,
		user.Data,
		user.Email,
		user.Phone,
		func(ctx context.Context) (*EncryptedCode, error) {
			return c.newEmailCode(ctx, c.eventstore.Filter, c.userEncryption) //nolint:staticcheck
		},
	)
	if err != nil {
		return nil, err
	}
	if codeEmail != "" {
		user.ReturnCodeEmail = &codeEmail
	}
	if codePhone != "" {
		user.ReturnCodePhone = &codePhone
	}
	return c.pushAppendAndReduceDetails(ctx, writeModel, events...)
}

func (c *Commands) DeleteSchemaUser(ctx context.Context, resourceOwner, id string) (*domain.ObjectDetails, error) {
	if id == "" {
		return nil, zerrors.ThrowInvalidArgument(nil, "COMMAND-Vs4wJCME7T", "Errors.IDMissing")
	}
	writeModel, err := c.getSchemaUserWMForState(ctx, resourceOwner, id)
	if err != nil {
		return nil, err
	}

	events, err := writeModel.NewDelete(ctx)
	if err != nil {
		return nil, err
	}

	return c.pushAppendAndReduceDetails(ctx, writeModel, events...)
}

type ChangeSchemaUser struct {
	schemaWriteModel *UserSchemaWriteModel

	ResourceOwner string
	ID            string

	SchemaUser *SchemaUser

	Email           *Email
	ReturnCodeEmail *string
	Phone           *Phone
	ReturnCodePhone *string
}

type SchemaUser struct {
	SchemaID string
	Data     json.RawMessage
}

func (s *ChangeSchemaUser) Valid() (err error) {
	if s.ID == "" {
		return zerrors.ThrowInvalidArgument(nil, "COMMAND-gEJR1QOGHb", "Errors.IDMissing")
	}
	if s.Email != nil && s.Email.Address != "" {
		if err := s.Email.Validate(); err != nil {
			return err
		}
	}

	if s.Phone != nil && s.Phone.Number != "" {
		if s.Phone.Number, err = s.Phone.Number.Normalize(); err != nil {
			return err
		}
	}

	return nil
}

func (c *Commands) ChangeSchemaUser(ctx context.Context, user *ChangeSchemaUser) (*domain.ObjectDetails, error) {
	if err := user.Valid(); err != nil {
		return nil, err
	}

	writeModel, err := c.getSchemaUserWMByID(ctx, user.ResourceOwner, user.ID)
	if err != nil {
		return nil, err
	}

	// use already used schemaID, if no new schemaID is defined
	schemaID := writeModel.SchemaID
	if user.SchemaUser != nil && user.SchemaUser.SchemaID != "" {
		schemaID = user.SchemaUser.SchemaID
	}

	var schemaWM *UserSchemaWriteModel
	if user.SchemaUser != nil {
		schemaWriteModel, err := existingSchema(ctx, c, "", schemaID)
		if err != nil {
			return nil, err
		}
		schemaWM = schemaWriteModel
	}

	events, codeEmail, codePhone, err := writeModel.NewUpdate(ctx,
		schemaWM,
		user.SchemaUser,
		user.Email,
		user.Phone,
		func(ctx context.Context) (*EncryptedCode, error) {
			return c.newEmailCode(ctx, c.eventstore.Filter, c.userEncryption) //nolint:staticcheck
		},
	)
	if err != nil {
		return nil, err
	}

	if codeEmail != "" {
		user.ReturnCodeEmail = &codeEmail
	}
	if codePhone != "" {
		user.ReturnCodePhone = &codePhone
	}
	return c.pushAppendAndReduceDetails(ctx, writeModel, events...)
}

func (c *Commands) getSchemaUserWMByID(ctx context.Context, resourceOwner, id string) (*UserV3WriteModel, error) {
	writeModel := NewUserV3WriteModel(resourceOwner, id, c.checkPermission)
	if err := c.eventstore.FilterToQueryReducer(ctx, writeModel); err != nil {
		return nil, err
	}
	return writeModel, nil
}

func existingSchema(ctx context.Context, c *Commands, resourceOwner, id string) (*UserSchemaWriteModel, error) {
	writeModel, err := c.getSchemaWriteModelByID(ctx, resourceOwner, id)
	if err != nil {
		return nil, err
	}
	if !writeModel.Exists() {
		return nil, zerrors.ThrowNotFound(nil, "COMMAND-VLDTtxT3If", "Errors.UserSchema.NotExists")
	}
	return writeModel, nil
}
