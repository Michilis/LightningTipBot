package cashu

import (
    "errors"
    "github.com/cashu-ts/cashu-ts"
)

var (
    mintUrl = "https://your-cashu-mint-url"
    wallet  = cashuts.NewCashuWallet(cashuts.NewCashuMint(mintUrl))
)

func DecodeToken(token string) (cashuts.Token, error) {
    decoded, err := wallet.DecodeToken(token)
    if err != nil {
        return cashuts.Token{}, errors.New("invalid Cashu token")
    }
    return decoded, nil
}

func RedeemToken(token string, userLightningAddress string) (string, error) {
    proofs, err := wallet.Receive(token)
    if err != nil {
        return "", errors.New("failed to receive Cashu token")
    }
    invoice, err := wallet.CreateInvoice(proofs.Amount, userLightningAddress)
    if err != nil {
        return "", errors.New("failed to create invoice")
    }
    result, err := wallet.PayLnInvoiceWithToken(invoice.PaymentRequest, token)
    if err != nil {
        return "", errors.New("failed to pay Lightning invoice with Cashu token")
    }
    return result, nil
}
