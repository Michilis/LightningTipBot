# Cashu Redeem Service

A microservice for redeeming Cashu tokens in the LightningTipBot.

## Setup

1. Install dependencies:
```bash
npm install
```

2. Set environment variables:
```bash
export MINT_URL=https://mint.0xchat.com  # or your preferred mint
export PORT=3333  # optional, defaults to 3333
```

3. Start the service:
```bash
npm start
```

## API Endpoints

### POST /redeem
Redeems a Cashu token and returns the amount.

Request:
```json
{
  "token": "cashuA1..."
}
```

Response:
```json
{
  "success": true,
  "amount": 1300
}
```

### POST /decode
Decodes a Cashu token and returns its mint + proofs.

Request:
```json
{
  "token": "cashuA1..."
}
```

Response:
```json
{
  "success": true,
  "decoded": {
    "mint": "...",
    "proofs": [...]
  }
}
```

### GET /health
Health check endpoint.

Response:
```json
{
  "status": "ok"
}
```

## Security Features

- Tracks redeemed tokens to prevent double-spending
- Validates token structure and mint
- Rate limiting (TODO)
- Token expiration (TODO) 