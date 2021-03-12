#!/bin/sh
BINDIR="/usr/local/bin"
echo "Copy centrify application launcher"
cp ${BINDIR}/centrify-app-launcher /centrify/bin/
echo "Injecting credentials..."
if [ "$VAULT_AUTHTYPE" = "oauth" ]; then
    # Use fake token string since the binary will try to get it from /var/secrets/oauthtoken inside container
    ${BINDIR}/centrify-secret-injector -auth oauth -url $VAULT_URL -appid $VAULT_APPID -scope $VAULT_SCOPE -token "faketoken"
fi