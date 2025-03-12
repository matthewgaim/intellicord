package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/matthewgaim/intellicord/internal/db"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/product"
	"github.com/stripe/stripe-go/v81/subscription"
)

func invoicePaymentSucceeded(event stripe.Event) (error, int) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		log.Printf("Error parsing webhook JSON: %v\n", err)
		return err, http.StatusBadRequest
	}

	subscriptionID := invoice.Subscription.ID
	if subscriptionID == "" {
		return errors.New("No subscription found in invoice"), http.StatusBadRequest
	}
	sub, err := subscription.Get(subscriptionID, nil)
	if err != nil {
		return errors.New("Subscription not found"), http.StatusBadRequest
	}
	discordID := sub.Metadata["discord_id"]
	if discordID == "" {
		return errors.New("Discord ID not found in metadata"), http.StatusBadRequest
	}
	if len(sub.Items.Data) == 0 {
		return errors.New("No items found in subscription"), http.StatusBadRequest
	}

	priceID := sub.Items.Data[0].Price.ID
	productID := sub.Items.Data[0].Plan.Product.ID
	product, err := product.Get(productID, nil)
	if err != nil {
		return errors.New("Product not found from given subscription"), http.StatusInternalServerError
	}
	planName := product.Name

	// Update the user's subscription in the database
	err = db.UpdateUsersPaidPlanStatus(discordID, priceID, planName)
	if err != nil {
		return errors.New("Failed to update user plan status"), http.StatusInternalServerError
	}
	return nil, 0
}

func customerSubscriptionDeleted(event stripe.Event) (error, int) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return errors.New("Error parsing webhook JSON"), http.StatusBadRequest
	}

	discordID := sub.Metadata["discord_id"]
	if discordID == "" {
		return errors.New("Discord ID not found in metadata"), http.StatusBadRequest
	}

	// Remove user's paid plan status from the database
	err := db.UpdateUsersPaidPlanStatus(discordID, "", "free")
	if err != nil {
		return errors.New("Failed to remove user plan status from database"), http.StatusInternalServerError
	}

	log.Printf("Subscription canceled for Discord ID: %s", discordID)
	return nil, 0
}
