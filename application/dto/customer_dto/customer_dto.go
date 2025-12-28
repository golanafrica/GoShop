package dto

import (
	"Goshop/domain/entity"
	"errors"
)

type CustomerRequestDto struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

type CustomerResponseDto struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

func ToCustomerRequest(c *entity.Customer) *CustomerRequestDto {
	if c == nil {
		return nil
	}

	return &CustomerRequestDto{
		FirstName: c.FirstName,
		LastName:  c.LastName,
		Email:     c.Email,
	}
}

func ToCustomerEntity(req *CustomerRequestDto) *entity.Customer {
	return &entity.Customer{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
	}
}

func ToCustomerResponse(c *entity.Customer) *CustomerResponseDto {
	return &CustomerResponseDto{
		ID:        c.ID,
		FirstName: c.FirstName,
		LastName:  c.LastName,
		Email:     c.Email,
	}
}

func ToCustomerResponses(customers []*entity.Customer) []*CustomerResponseDto {
	responses := make([]*CustomerResponseDto, len(customers))
	for i, c := range customers {
		responses[i] = ToCustomerResponse(c)
	}
	return responses
}

func (c *CustomerRequestDto) Validate() error {
	if c.FirstName == "" {
		return errors.New("customer name is required")
	}
	if c.Email == "" {
		return errors.New("email is required")
	}
	if c.LastName == "" {
		return errors.New("phone number is required")
	}

	return nil
}
