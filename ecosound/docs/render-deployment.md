# Deploy EcoSound on Render

`render.yaml` creates two Starter services: the Next.js frontend and a Docker Go
API with FFmpeg/FFprobe. A 1 GB persistent disk stores SQLite and audio. BirdNET
and Qwen remain on their existing Modal deployments. There is no new GPU service
on Render.

## Deploy

1. Commit and push the EcoSound changes, including `render.yaml`,
   `backend/Dockerfile` and `backend/.dockerignore`. Do not commit `.env`.
2. In Render, choose **New → Blueprint**, connect this Git repository and choose
   your branch. Set **Blueprint Path** to **`ecosound/render.yaml`**.
3. Supply `MODAL_PROXY_TOKEN_ID`, `MODAL_PROXY_TOKEN_SECRET`,
   `QWEN_MODAL_PROXY_TOKEN_ID` and `QWEN_MODAL_PROXY_TOKEN_SECRET` from your local
   `.env` in the secure Render fields. The Qwen pair may be the same as BirdNET
   if authorized for both Modal endpoints.
4. For `FRONTEND_ORIGIN`, initially use `https://ecosound-web.onrender.com`.
   Review the two paid services and disk, then deploy the Blueprint.
5. When Render assigns the frontend's public URL, set the API service's
   `FRONTEND_ORIGIN` to that **exact** URL, without a trailing slash. Render may
   add a suffix to the service hostname. Save and redeploy the API if needed.
6. Open the frontend's HTTPS URL. The frontend's `NEXT_PUBLIC_API_URL` is copied
   from the API's `RENDER_EXTERNAL_URL` by the Blueprint and embedded at build
   time. If the API URL changes, sync the Blueprint and rebuild the frontend.

## Verify

- The API's `/health` should return `{"inference":"configured","status":"ok"}`.
  This verifies the running API, not Modal model readiness.
- Upload a short bird recording, review candidates, and save a discovery.
- Ask for a species explanation and build a nature mission. A Qwen cold start
  may take several minutes; subsequent requests should be quicker while warm.
- Browser uploads need the exact CORS origin from step 5. A wrong origin can
  appear as a network error and queue the audio locally.
- Localhost recordings and browser offline data do not transfer automatically
  to the new HTTPS origin. The Render database starts with fresh user data.

## Storage and costs

These are paid Starter services, with additional persistent disk charges. Your
Render promotional balance is credit applied to eligible usage, not an unlimited
hosting plan. Check the current dashboard estimate before deploying.

Only `/var/data` survives API restarts and deploys. Keep one API instance because
SQLite and the attached disk belong to that instance. Disk-backed redeploys have
a short interruption. The disk can be enlarged later if audio fills it.

EcoSound currently has a shared collection and no user accounts: the deployed
site is a shared hackathon demo, and stored recordings can be accessed through
its API. Use recordings intended for that demo.

References: [Blueprints](https://render.com/docs/infrastructure-as-code),
[YAML specification](https://render.com/docs/blueprint-spec),
[persistent disks](https://render.com/docs/disks).
