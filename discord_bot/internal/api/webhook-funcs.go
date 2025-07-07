package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

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

	subStartTimestamp := sub.CurrentPeriodStart
	subRenewalTimestamp := sub.CurrentPeriodEnd

	subStartDate := time.Unix(subStartTimestamp, 0)
	subRenewalDate := time.Unix(subRenewalTimestamp, 0)

	subStartStr := subStartDate.String()
	subRenewalStr := subRenewalDate.String()
	log.Printf("Subscription %s:\nStart Date: %s\nRenew Date %s\n", subscriptionID, subStartStr, subRenewalStr)

	planName := product.Name

	// Update the user's subscription in the database
	err = db.UpdateUsersPaidPlanStatus(discordID, priceID, planName, subStartDate, subRenewalDate, sub.Customer.ID)
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
	now := time.Now()
	renewalDate := now.AddDate(0, 1, 0)
	err := db.UpdateUsersPaidPlanStatus(discordID, "", "free", now, renewalDate, "")
	if err != nil {
		return errors.New("Failed to remove user plan status from database"), http.StatusInternalServerError
	}

	log.Printf("Subscription canceled for Discord ID: %s", discordID)
	return nil, 0
}

func customerSubscriptionUpdated(event stripe.Event) (error, int) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return errors.New("Error parsing webhook JSON"), http.StatusBadRequest
	}

	discordID := sub.Metadata["discord_id"]
	if discordID == "" {
		return errors.New("Discord ID not found in metadata"), http.StatusBadRequest
	}

	subStartTimestamp := sub.CurrentPeriodStart
	subRenewalTimestamp := sub.CurrentPeriodEnd
	subStartDate := time.Unix(subStartTimestamp, 0)
	subRenewalDate := time.Unix(subRenewalTimestamp, 0)

	// Check if the plan changed
	if len(sub.Items.Data) == 0 {
		log.Println("No items found in subscription")
		return nil, 0
	}

	priceID := sub.Items.Data[0].Price.ID
	productID := sub.Items.Data[0].Plan.Product.ID
	product, err := product.Get(productID, nil)
	if err != nil {
		return errors.New("Product not found from given subscription"), http.StatusInternalServerError
	}

	planName := product.Name

	// If the plan has changed, ensure the remaining period is honored
	if sub.Status == stripe.SubscriptionStatusActive {
		err := db.UpdateUsersPaidPlanStatus(discordID, priceID, planName, subStartDate, subRenewalDate, sub.Customer.ID)
		if err != nil {
			return errors.New("Failed to update user plan status"), http.StatusInternalServerError
		}
		log.Printf("Plan updated for Discord ID: %s, New Plan: %s, Renewal Date: %s", discordID, planName, subRenewalDate.String())
	} else {
		log.Printf("Subscription update ignored for Discord ID: %s (status: %s)", discordID, sub.Status)
	}

	return nil, 0
}
