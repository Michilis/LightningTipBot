import express from 'express';
import bodyParser from 'body-parser';
import { CashuMint, CashuWallet, getDecodedToken } from '@cashu/cashu-ts';
import dotenv from 'dotenv';

// Load environment variables
dotenv.config();

const app = express();
const port = process.env.PORT || 3333;
const debug = process.env.DEBUG === 'true';
const maxRedeemedTokens = parseInt(process.env.MAX_REDEEMED_TOKENS || '10000');
const tokenExpirationHours = parseInt(process.env.TOKEN_EXPIRATION_HOURS || '24');

app.use(bodyParser.json());

// Store for tracking redeemed tokens with timestamps
const redeemedTokens = new Map();

// Initialize Cashu wallet
const mintUrl = process.env.MINT_URL || 'https://kashu.me';
const wallet = new CashuWallet(new CashuMint(mintUrl));

// Cleanup expired tokens periodically
setInterval(() => {
    if (tokenExpirationHours > 0) {
        const now = Date.now();
        for (const [token, timestamp] of redeemedTokens.entries()) {
            if (now - timestamp > tokenExpirationHours * 60 * 60 * 1000) {
                redeemedTokens.delete(token);
            }
        }
    }
}, 60 * 60 * 1000); // Check every hour

// Helper function to validate token format
function isValidTokenFormat(token) {
    // Match both v1 and v3 token formats
    return /^cashu[abAB][a-zA-Z0-9-_]+$/.test(token);
}

// Helper function to get token mint URL
async function getTokenMintUrl(token) {
    try {
        const decoded = await getDecodedToken(token);
        if (!decoded) {
            if (debug) {
                console.error('Failed to decode token');
            }
            return null;
        }

        // Handle both v1 and v3 token formats
        if (decoded.mint) {
            // v3 format
            return decoded.mint;
        } else if (decoded.token && decoded.token[0] && decoded.token[0].mint) {
            // v1 format
            return decoded.token[0].mint;
        }

        if (debug) {
            console.error('Invalid token structure:', decoded);
        }
        return null;
    } catch (error) {
        if (debug) {
            console.error('Error getting token mint URL:', error);
        }
        return null;
    }
}

// Helper function to decode token
async function decodeToken(token) {
    try {
        const decoded = await getDecodedToken(token);
        if (!decoded) {
            throw new Error('Failed to decode token');
        }

        // Handle both v1 and v3 token formats
        if (decoded.proofs) {
            // v3 format
            return {
                proofs: decoded.proofs,
                mint: decoded.mint
            };
        } else if (decoded.token && decoded.token[0]) {
            // v1 format
            return {
                proofs: decoded.token[0].proofs,
                mint: decoded.token[0].mint
            };
        }

        throw new Error('Invalid token structure');
    } catch (error) {
        if (debug) {
            console.error('Error decoding token:', error);
        }
        throw error;
    }
}

// Redeem endpoint
app.post('/redeem', async (req, res) => {
    try {
        const { token } = req.body;
        
        if (!token) {
            return res.status(400).json({ 
                success: false, 
                error: 'Token is required' 
            });
        }

        if (debug) {
            console.log('Received token:', token);
        }

        // Validate token format
        if (!isValidTokenFormat(token)) {
            if (debug) {
                console.error('Invalid token format:', token);
            }
            return res.status(400).json({ 
                success: false, 
                error: 'Invalid token format. Must be a valid Cashu token' 
            });
        }

        // Check if token was already redeemed
        if (redeemedTokens.has(token)) {
            return res.status(400).json({ 
                success: false, 
                error: 'Token already redeemed' 
            });
        }

        // Get token mint URL
        const tokenMintUrl = await getTokenMintUrl(token);
        if (!tokenMintUrl) {
            if (debug) {
                console.error('Failed to get token mint URL for:', token);
            }
            return res.status(400).json({ 
                success: false, 
                error: 'Invalid token format or unable to decode' 
            });
        }

        if (debug) {
            console.log('Token mint URL:', tokenMintUrl);
        }

        // Decode and verify token
        let decoded;
        try {
            decoded = await decodeToken(token);
            if (debug) {
                console.log('Decoded token:', decoded);
            }
        } catch (error) {
            if (debug) {
                console.error('Error decoding token:', error);
            }
            return res.status(400).json({ 
                success: false, 
                error: 'Failed to decode token: ' + error.message 
            });
        }

        if (!decoded.proofs || !Array.isArray(decoded.proofs) || decoded.proofs.length === 0) {
            if (debug) {
                console.error('Invalid decoded token structure:', decoded);
            }
            return res.status(400).json({ 
                success: false, 
                error: 'Invalid token format' 
            });
        }

        // Calculate total amount
        const amount = decoded.proofs.reduce((sum, proof) => sum + (proof.amount || 0), 0);
        if (amount <= 0) {
            if (debug) {
                console.error('Token has no value:', decoded);
            }
            return res.status(400).json({ 
                success: false, 
                error: 'Token has no value' 
            });
        }

        // Mark token as redeemed with timestamp
        redeemedTokens.set(token, Date.now());

        // Cleanup if we have too many tokens
        if (redeemedTokens.size > maxRedeemedTokens) {
            const oldestToken = redeemedTokens.entries().next().value[0];
            redeemedTokens.delete(oldestToken);
        }

        if (debug) {
            console.log(`Redeemed token: ${token}, amount: ${amount}`);
        }

        res.json({
            success: true,
            amount: amount,
            mint_url: tokenMintUrl
        });
    } catch (error) {
        console.error('Error redeeming token:', error);
        res.status(500).json({ 
            success: false, 
            error: 'Failed to redeem token: ' + error.message 
        });
    }
});

// Decode endpoint
app.post('/decode', async (req, res) => {
    try {
        const { token } = req.body;
        
        if (!token) {
            return res.status(400).json({ 
                success: false, 
                error: 'Token is required' 
            });
        }

        // Validate token format
        if (!isValidTokenFormat(token)) {
            return res.status(400).json({ 
                success: false, 
                error: 'Invalid token format. Must start with cashuA or cashuB' 
            });
        }

        let decoded;
        try {
            decoded = await wallet.decodeToken(token);
        } catch (error) {
            if (debug) {
                console.error('Error decoding token:', error);
            }
            return res.status(400).json({ 
                success: false, 
                error: 'Failed to decode token: ' + error.message 
            });
        }

        if (!decoded) {
            return res.status(400).json({ 
                success: false, 
                error: 'Invalid token format' 
            });
        }

        // Get token mint URL
        const tokenMintUrl = await getTokenMintUrl(token);

        res.json({
            success: true,
            decoded: decoded,
            mint_url: tokenMintUrl,
            format: token.startsWith('cashuA') ? 'cashuA' : 'cashuB'
        });
    } catch (error) {
        console.error('Error decoding token:', error);
        res.status(500).json({ 
            success: false, 
            error: 'Failed to decode token: ' + error.message 
        });
    }
});

// Add /pay endpoint to handle invoice payments
app.post('/pay', async (req, res) => {
    const { payment_request } = req.body;
    
    if (!payment_request) {
        return res.status(400).json({ 
            success: false, 
            error: 'Payment request is required' 
        });
    }

    try {
        // Get the mint URL from the last redeemed token
        const lastToken = Array.from(redeemedTokens.keys()).pop();
        if (!lastToken) {
            return res.status(400).json({ 
                success: false, 
                error: 'No token has been redeemed yet' 
            });
        }

        const tokenMintUrl = await getTokenMintUrl(lastToken);
        if (!tokenMintUrl) {
            return res.status(400).json({ 
                success: false, 
                error: 'Could not determine mint URL' 
            });
        }

        if (debug) {
            console.log('Paying invoice:', payment_request);
            console.log('Using mint URL:', tokenMintUrl);
        }

        // First, melt the token at the mint
        try {
            const decoded = await decodeToken(lastToken);
            if (!decoded || !decoded.proofs) {
                throw new Error('Invalid token structure');
            }

            // Calculate total amount
            const amount = decoded.proofs.reduce((sum, proof) => sum + (proof.amount || 0), 0);
            
            if (debug) {
                console.log('Melting token for amount:', amount);
            }

            // Melt the token at the mint
            const meltResponse = await fetch(`${tokenMintUrl}/melt`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    proofs: decoded.proofs,
                    pr: payment_request
                })
            });

            if (!meltResponse.ok) {
                const error = await meltResponse.text();
                if (debug) {
                    console.error('Error melting token:', error);
                }
                let errorMessage = 'Failed to melt token at mint';
                try {
                    const errorJson = JSON.parse(error);
                    if (errorJson.detail) {
                        errorMessage = errorJson.detail;
                    }
                } catch (e) {
                    // If we can't parse the error as JSON, use the raw error text
                    errorMessage = error;
                }
                return res.status(400).json({ 
                    success: false, 
                    error: errorMessage
                });
            }

            const meltResult = await meltResponse.json();
            if (!meltResult.paid) {
                if (debug) {
                    console.error('Token melted but payment failed:', meltResult);
                }
                return res.status(400).json({ 
                    success: false, 
                    error: 'Token melted but payment failed' 
                });
            }

            if (debug) {
                console.log('Token melted successfully:', meltResult);
            }

            return res.json({ 
                success: true, 
                message: 'Invoice paid successfully',
                change: meltResult.change
            });
        } catch (error) {
            if (debug) {
                console.error('Error in token melting:', error);
            }
            return res.status(400).json({ 
                success: false, 
                error: 'Failed to process token: ' + error.message 
            });
        }
    } catch (error) {
        if (debug) {
            console.error('Error in /pay:', error);
        }
        return res.status(500).json({ 
            success: false, 
            error: 'Internal server error' 
        });
    }
});

// Health check endpoint
app.get('/health', (req, res) => {
    res.json({ 
        status: 'ok',
        mint_url: mintUrl,
        redeemed_tokens: redeemedTokens.size,
        debug_mode: debug,
        max_tokens: maxRedeemedTokens,
        token_expiration_hours: tokenExpirationHours
    });
});

// Start the server
app.listen(port, () => {
    console.log(`Cashu redeem service listening on port ${port}`);
    console.log(`Using mint URL: ${mintUrl}`);
    console.log(`Debug mode: ${debug}`);
    console.log(`Token expiration: ${tokenExpirationHours} hours`);
}); 