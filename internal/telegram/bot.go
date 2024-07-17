package telegram

import (
    "strings"
    "github.com/LightningTipBot/LightningTipBot/cashu"
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func handleMessage(msg *tgbotapi.Message) {
    if strings.HasPrefix(msg.Text, "cashu-token:") {
        cashuToken := strings.TrimPrefix(msg.Text, "cashu-token:")
        cashuToken = strings.TrimSpace(cashuToken)

        if msg.Chat.IsPrivate() {
            // Handle DM
            userLightningAddress, err := getUserLightningAddress(msg.From.ID)
            if err != nil {
                bot.Send(msg.Chat.ID, "Failed to retrieve your Lightning address.")
                return
            }
            result, err := cashu.RedeemToken(cashuToken, userLightningAddress)
            if err != nil {
                bot.Send(msg.Chat.ID, "Failed to redeem Cashu token.")
                return
            }
            bot.Send(msg.Chat.ID, "Cashu token successfully redeemed to your Lightning wallet.")
        } else {
            // Handle group message
            redeemButton := tgbotapi.NewInlineKeyboardButtonData("Redeem Cashu Token", "redeem:"+cashuToken)
            markup := tgbotapi.NewInlineKeyboardMarkup([]tgbotapi.InlineKeyboardButton{redeemButton})
            msg := tgbotapi.NewMessage(msg.Chat.ID, "A Cashu token has been sent. Click the button to redeem it.")
            msg.ReplyMarkup = &markup
            bot.Send(msg)
        }
    }
}
