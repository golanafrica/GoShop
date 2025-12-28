package mapper

import (
	orderitemdto "Goshop/application/dto/orderItem_dto"
	orderdto "Goshop/application/dto/order_dto"
	"Goshop/domain/entity"
)

func ToOrderEntity(req *orderdto.OrderRequestDto) *entity.Order {
	items := make([]*entity.OrderItem, len(req.Items))

	for i, it := range req.Items {
		items[i] = &entity.OrderItem{
			ProductID: it.ProductID,
			Quantity:  int(it.Quantity),
		}
	}

	return &entity.Order{
		CustomerID: req.CustomerID,
		Items:      items,
	}
}

func ToOrderResponse(order *entity.Order) *orderdto.OrderResponseDto {
	items := make([]*orderitemdto.OrderItemResponseDto, len(order.Items))

	for i, it := range order.Items {
		items[i] = &orderitemdto.OrderItemResponseDto{
			ID:            it.ID,
			ProductID:     it.ProductID,
			Quantity:      int(it.Quantity),
			PriceCents:    it.PriceCents,
			SubTotalCents: int64(it.SubTotal_Cents),
		}
	}

	return &orderdto.OrderResponseDto{
		ID:         order.ID,
		CustomerID: order.CustomerID,
		TotalCents: order.TotalCents,
		Status:     order.Status,
		Items:      items,
	}
}
