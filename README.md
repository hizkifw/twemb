# twemb

Discord bot that substitutes all Twitter/X links to vxtwitter.

```sh
export DISCORD_BOT_TOKEN="..."
go run .
```

## Deployment

Pushing to `main` runs `.github/workflows/deploy.yml`, which connects to the
server as `twemb-deploy`. That user's SSH key is pinned to
`/usr/local/sbin/twemb-deploy` (a copy of `deploy/twemb-deploy`), which pulls
`main`, rebuilds and restarts `twemb.service`. Changes to the script must be
copied to the server by hand.
