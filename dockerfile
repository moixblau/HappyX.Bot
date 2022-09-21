FROM node:16
WORKDIR /usr/src/app
COPY src/package*.json ./
RUN npm install

# If you are building your code for production
# RUN npm ci --only=production
COPY src .
EXPOSE 80
CMD [ "node", "app.js" ]