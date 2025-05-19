package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/cashu"
	"github.com/LightningTipBot/LightningTipBot/internal/errors"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	"github.com/LightningTipBot/LightningTipBot/internal/configuration"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/pkg/lightning"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v3"
)

func (bot *TipBot) anyTextHandler(ctx intercept.Context) (intercept.Context, error) {
	m := ctx.Message()
	if m.Chat.Type != tb.ChatPrivate {
		return ctx, errors.Create(errors.NoPrivateChatError)
	}

	// check if user is in Database, if not, initialize wallet
	user := LoadUser(ctx)
	if user.Wallet == nil || !user.Initialized {
		return bot.startHandler(ctx)
	}

	// check if the user clicked on the balance button
	if strings.HasPrefix(m.Text, MainMenuCommandBalance) {
		bot.tryDeleteMessage(m)
		// overwrite the message text so it doesn't cause an infinite loop
		// because balanceHandler calls anyTextHAndler...
		m.Text = ""
		return bot.balanceHandler(ctx)
	}

	// check for Cashu token
	if cashu.ContainsCashuToken(m.Text) {
		token := cashu.ExtractCashuToken(m.Text)
		if token == "" {
			bot.trySendMessage(m.Sender, "❌ Invalid Cashu token format. Please send a valid Cashu token.")
			return ctx, nil
		}

		// Send initial processing message
		processingMsg := bot.trySendMessage(m.Sender, Translate(ctx, "cashuProcessingMessage"))

		// Get the response details
		response, err := cashu.GetRedeemResponse(token, configuration.Get().Cashu.ServiceURL)
		if err != nil {
			log.Errorf("[Cashu] Error getting redeem response: %v", err)
			bot.tryDeleteMessage(processingMsg)
			bot.trySendMessage(m.Sender, Translate(ctx, "cashuNoRedemptionDetailsMessage"))
			return ctx, err
		}

		// Add a small fee buffer to account for routing fees
		feeBuffer := response.Fee // Use the fee from the response
		invoiceAmount := response.NetAmount // Use the net amount after fees
		if invoiceAmount <= 0 {
			bot.tryDeleteMessage(processingMsg)
			bot.trySendMessage(m.Sender, Translate(ctx, "cashuAmountTooSmallMessage"))
			return ctx, fmt.Errorf("token amount too small")
		}

		// Create an invoice for the user to receive the payment
		invoiceParams := lnbits.InvoiceParams{
			Out:    false, // false means receiving payment
			Amount: invoiceAmount,
			Memo:   "Cashu token redemption",
		}
		invoice, err := user.Wallet.Invoice(invoiceParams, bot.Client)
		if err != nil {
			log.Errorf("[Cashu] Error creating invoice: %v", err)
			bot.tryDeleteMessage(processingMsg)
			bot.trySendMessage(m.Sender, Translate(ctx, "cashuCreateInvoiceFailedMessage"))
			return ctx, err
		}

		// Send the invoice to the Cashu service to be paid by the mint
		err = cashu.PayInvoice(invoice.PaymentRequest, configuration.Get().Cashu.ServiceURL)
		if err != nil {
			log.Errorf("[Cashu] Error paying invoice: %v", err)
			bot.tryDeleteMessage(processingMsg)
			
			// Extract the actual error message from the Cashu service
			errorMsg := err.Error()
			if strings.Contains(errorMsg, "error from Cashu service:") {
				// Extract the JSON error message
				parts := strings.Split(errorMsg, "error from Cashu service:")
				if len(parts) > 1 {
					errorMsg = strings.TrimSpace(parts[1])
					// Try to parse the JSON error
					var errorResponse struct {
						Error string `json:"error"`
					}
					if err := json.Unmarshal([]byte(errorMsg), &errorResponse); err == nil {
						errorMsg = errorResponse.Error
					}
				}
			}

			// Format the error message for display
			displayMsg := fmt.Sprintf(Translate(ctx, "cashuProcessFailedMessage"), errorMsg)
			bot.trySendMessage(m.Sender, displayMsg)
			return ctx, err
		}

		// Delete processing message and send success message
		bot.tryDeleteMessage(processingMsg)
		feeText := "sat"
		if feeBuffer > 1 {
			feeText = "sats"
		}
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "cashuSuccessMessage"), 
			invoiceAmount,
			response.MintURL,
			feeBuffer,
			feeText))

		// Refresh user's balance
		balance, err := bot.GetUserBalance(user)
		if err != nil {
			log.Errorf("[Cashu] Error getting user balance: %v", err)
		} else {
			bot.trySendMessage(m.Sender, fmt.Sprintf("💰 Your new balance: %d sat", balance))
		}

		return ctx, nil
	}

	// could be an invoice
	anyText := strings.ToLower(m.Text)
	if lightning.IsInvoice(anyText) {
		m.Text = "/pay " + anyText
		return bot.payHandler(ctx)
	}
	if lightning.IsLnurl(anyText) {
		m.Text = "/lnurl " + anyText
		return bot.lnurlHandler(ctx)
	}
	if c := stateCallbackMessage[user.StateKey]; c != nil {
		return c(ctx)
	}
	return ctx, nil
}

type EnterUserStateData struct {
	ID              string `json:"ID"`              // holds the ID of the tx object in bunt db
	Type            string `json:"Type"`            // holds type of the tx in bunt db (needed for type checking)
	Amount          int64  `json:"Amount"`          // holds the amount entered by the user mSat
	AmountMin       int64  `json:"AmountMin"`       // holds the minimum amount that needs to be entered mSat
	AmountMax       int64  `json:"AmountMax"`       // holds the maximum amount that needs to be entered mSat
	OiringalCommand string `json:"OiringalCommand"` // hold the originally entered command for evtl later use
}

func (bot *TipBot) askForUser(ctx context.Context, id string, eventType string, originalCommand string) (enterUserStateData *EnterUserStateData, err error) {
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return // errors.New("user has no wallet"), 0
	}
	enterUserStateData = &EnterUserStateData{
		ID:              id,
		Type:            eventType,
		OiringalCommand: originalCommand,
	}
	// set LNURLPayParams in the state of the user
	stateDataJson, err := json.Marshal(enterUserStateData)
	if err != nil {
		log.Errorln(err)
		return
	}
	SetUserState(user, bot, lnbits.UserEnterUser, string(stateDataJson))
	// Let the user enter a user and return
	bot.trySendMessage(user.Telegram, Translate(ctx, "enterUserMessage"), tb.ForceReply)
	return
}

// enterAmountHandler is invoked in anyTextHandler when the user needs to enter an amount
// the amount is then stored as an entry in the user's stateKey in the user database
// any other ctx that relies on this, needs to load the resulting amount from the database
func (bot *TipBot) enterUserHandler(ctx intercept.Context) (intercept.Context, error) {
	m := ctx.Message()
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return ctx, errors.Create(errors.UserNoWalletError)
	}

	if !(user.StateKey == lnbits.UserEnterUser) {
		ResetUserState(user, bot)
		return ctx, errors.Create(errors.InvalidSyntaxError)
	}
	if len(m.Text) < 4 || strings.HasPrefix(m.Text, "/") || m.Text == SendMenuCommandEnter {
		ResetUserState(user, bot)
		return ctx, errors.Create(errors.InvalidSyntaxError)
	}

	var EnterUserStateData EnterUserStateData
	err := json.Unmarshal([]byte(user.StateData), &EnterUserStateData)
	if err != nil {
		log.Errorf("[EnterUserHandler] %s", err.Error())
		ResetUserState(user, bot)
		return ctx, err
	}

	userstr := m.Text

	// find out which type the object in bunt has waiting for an amount
	// we stored this in the EnterAmountStateData before
	switch EnterUserStateData.Type {
	case "CreateSendState":
		m.Text = fmt.Sprintf("/send %s", userstr)
		return bot.sendHandler(ctx)
	default:
		ResetUserState(user, bot)
		return ctx, errors.Create(errors.InvalidSyntaxError)
	}
	return ctx, nil
}
