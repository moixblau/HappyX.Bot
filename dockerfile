FROM node:16
WORKDIR /usr/src/app
COPY src/package*.json ./
RUN npm install

ENV SLACK_BOT_TOKEN=xoxb-3479460306208-3459824605459-eGNvR4qcslsr1X3fZo7eKa22
ENV SLACK_SIGNING_SECRET=4186d7fec2d8c9439eefa08fe3d5f783
ENV API_HOST=http://happyx.moixblau.com:8083
ENV API_TOKEN=ALX

# If you are building your code for production
# RUN npm ci --only=production
COPY src .
EXPOSE 80
CMD [ "node", "app.js" ]