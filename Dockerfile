FROM golang:1.16.6-buster as backend
WORKDIR /go/src/powerbox-http-proxy
COPY . .
RUN go build

FROM node:16.4.2-buster as frontend
COPY . .
RUN npm install
RUN npm run build

FROM zenhack/sandstorm-http-bridge:276
COPY --from=backend /go/src/powerbox-http-proxy/powerbox-http-proxy /
COPY --from=frontend /build /build/
