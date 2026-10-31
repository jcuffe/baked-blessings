# Build from the repo root:  docker build -f apps/bakehouse/Dockerfile .
FROM node:22-alpine AS base
RUN corepack enable
WORKDIR /repo

FROM base AS build
COPY . .
RUN pnpm install --frozen-lockfile
RUN pnpm turbo run build --filter=@bb/bakehouse...
RUN pnpm deploy --filter=@bb/bakehouse --prod /out

FROM node:22-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production
COPY --from=build /out .
EXPOSE 3000
CMD ["npm", "run", "docker-start"]
