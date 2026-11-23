# Storefront container: the Oxygen worker bundle served by plain Node (container/server.mjs).
# One image runs on Lambda (through the Lambda Web Adapter), Fargate or Kubernetes.
# Build from the repo root:  docker build -f apps/storefront/Dockerfile .
FROM node:22-alpine AS build
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0
RUN corepack enable
WORKDIR /repo
COPY . .
RUN pnpm install --frozen-lockfile --filter "@bb/storefront..."
RUN pnpm turbo run build --filter="@bb/storefront..."

FROM node:22-alpine AS runtime
# Lambda Web Adapter: a Lambda extension that forwards invocations to the HTTP server.
# It only starts inside Lambda; elsewhere the image is an ordinary web server.
COPY --from=public.ecr.aws/awsguru/aws-lambda-adapter:1.1.0 /lambda-adapter /opt/extensions/lambda-adapter
WORKDIR /app
ENV NODE_ENV=production \
    PORT=8080 \
    AWS_LWA_READINESS_CHECK_PATH=/healthz \
    AWS_LWA_INVOKE_MODE=response_stream
RUN echo '{"type":"module","private":true}' > package.json
COPY --from=build /repo/apps/storefront/dist ./dist
COPY --from=build /repo/apps/storefront/container ./container
USER node
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
CMD ["node", "container/server.mjs"]
