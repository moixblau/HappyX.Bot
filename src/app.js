const { App } = require("@slack/bolt");
const nodeCron = require("node-cron");
const axios = require("axios");

const app = new App({
  token: process.env.SLACK_BOT_TOKEN,
  signingSecret: process.env.SLACK_SIGNING_SECRET,
});

const api = {
  host: process.env.API_HOST,
  token: process.env.API_TOKEN,
};

axios.defaults.baseURL = api.host;
axios.defaults.headers.common["Authorization"] = api.token;

// Schedule mood selection message
//const job = nodeCron.schedule("0 13 * * 1-5", function jobYouNeedToExecute() {
const job = nodeCron.schedule("* * * * *", function jobYouNeedToExecute() {
  axios
    .get("/User/GetUsers")
    .then((res) => {
      console.log(`Mood Scheduled send - Status: ${res.status}`);
      var users = res.data;
      users.forEach((user) => {
        app.client.chat
          .postMessage({
            channel: user.slackId,
            text: "¿Cómo te encuentras hoy?",
            blocks: [
              {
                type: "divider",
              },
              {
                type: "section",
                text: {
                  type: "mrkdwn",
                  text: "¿Cómo te encuentras hoy?",
                },
              },
              {
                type: "actions",
                elements: [
                  {
                    type: "button",
                    text: {
                      type: "plain_text",
                      text: ":disappointed:",
                    },
                    action_id: "mood-set-sad",
                  },
                  {
                    type: "button",
                    text: {
                      type: "plain_text",
                      text: ":pensive:",
                    },
                    action_id: "mood-set-unhappy",
                  },
                  {
                    type: "button",
                    text: {
                      type: "plain_text",
                      text: ":expressionless:",
                    },
                    action_id: "mood-set-indiferent",
                  },
                  {
                    type: "button",
                    text: {
                      type: "plain_text",
                      text: ":slightly_smiling_face:",
                    },
                    action_id: "mood-set-happy",
                  },
                  {
                    type: "button",
                    text: {
                      type: "plain_text",
                      text: ":star-struck:",
                    },
                    action_id: "mood-set-joyfull",
                  },
                ],
              },
            ],
          })
          .then((res) => {});
      });
    })
    .catch((err) => {
      console.error(err);
    });
});

// Slash commands
app.command("/joinx", async ({ body, ack, say }) => {
  await ack();

  var slack_id = body["user_id"];
  var slack_name = body["user_name"];

  var body = { slackId: slack_id, userName: slack_name };

  axios
    .post("/User/AddUser", body)
    .then((res) => {
      console.log(`User ${slack_name} joined - Status: ${res.status}!`);
      say("Welcome! 👋🤖");
    })
    .catch((err) => {
      say("Something goes wrong, try it later! 😖");
      console.error(err);
    });
});

app.command("/leavex", async ({ body, ack, say }) => {
  await ack();

  var slack_id = body["user_id"];
  var slack_name = body["user_name"];
  var body = { slackId: slack_id };

  axios
    .delete("/User/DeleteUser", {data: body}) // Don't work if I use body as parameter, you need to use data json.
    .then((res) => {
      console.log(`User ${slack_name} leave - Status: ${res.status}!`);
      say("Good bye! 👋🤖");
    })
    .catch((err) => {
      say("Something goes wrong, try it later! 😖");
      console.error(err);
    });
});

async function responseMood(res, ack, say, mood) {
  await ack();

  var channelId = res["channel"]["id"];
  var ts = res["message"]["ts"];

  await app.client.chat.delete({
    channel: channelId,
    ts: ts,
  });

  // Save user record
  var slack_id = res["user"]["id"];
  var slack_name =  res["user"]["name"];
  var body = { slackId: slack_id, moodName: mood };

  axios
    .post("/Record/AddRecord", body)
    .then((res) => {
      say("Thanks! 🤘");
      console.log(`${slack_name} today's mood is ${mood} - Status: ${res.status}!`)
    })
    .catch((err) => {
      console.error(err);
    });
}

// Moods reaction
app.action("mood-set-joyfull", async ({ body, ack, say }) => {
  await responseMood(body, ack, say, "joyful");
});
app.action("mood-set-happy", async ({ body, ack, say }) => {
  await responseMood(body, ack, say, "happy");
});
app.action("mood-set-indiferent", async ({ body, ack, say }) => {
  await responseMood(body, ack, say, "indifferent");
});
app.action("mood-set-unhappy", async ({ body, ack, say }) => {
  await responseMood(body, ack, say, "unhappy");
});
app.action("mood-set-sad", async ({ body, ack, say }) => {
  await responseMood(body, ack, say, "sad");
});

// Start server
(async () => {
  await app.start(process.env.PORT || 80);
  job.start();
  console.log("🤖 Bot is running!");
})();
