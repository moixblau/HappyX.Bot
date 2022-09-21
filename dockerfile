FROM node:16
WORKDIR /usr/src/app
COPY src/package*.json ./
RUN npm install

ENV SLACK_BOT_TOKEN=REMOVED
ENV SLACK_SIGNING_SECRET=REMOVED
ENV API_HOST=http://happyx.moixblau.com:8083
ENV API_TOKEN=REMOVED

# If you are building your code for production
# RUN npm ci --only=production
COPY src .
EXPOSE 80
CMD [ "node", "app.js" ]