FROM node:24-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
# ci rather than install: it builds exactly the tree the lockfile names and
# fails if package.json has drifted from it, which is the difference between
# an image that matches what CI tested and one that merely resolves today.
RUN npm ci --no-audit --no-fund
COPY . .
RUN npm run build

FROM node:24-alpine
WORKDIR /app
ENV NODE_ENV=production
COPY --from=build /app/package.json ./package.json
COPY --from=build /app/build ./build
EXPOSE 3000
CMD ["node", "build"]
