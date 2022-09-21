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
const job = nodeCron.schedule("0 13 * * 1-5", function jobYouNeedToExecute() {
  var users;

  axios
    .get("/User/GetUsers")
    .then((res) => {
      console.log(`Status: ${res.status}`);
      users = res.data;
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

  console.log(new Date().toLocaleString());
});

// Slash commands
app.command("/joinx", async ({ body, command, ack, say }) => {
  await ack();

  var slack_id = body["user_id"];
  var slack_name = body["user_name"];

  var body = { slackId: slack_id, userName: slack_name };

  axios
    .post("/User/AddUser", body)
    .then((res) => {
      say("Welcome! 👋🤖");
    })
    .catch((err) => {
      say("Something goes wrong, try it later! 😖");
      console.error(err);
    });
});

app.command("/leavex", async ({ body, command, ack, say }) => {
  await ack();

  var slack_id = body["user_id"];
  var body = { slackId: slack_id };

  axios
    .delete("/User/DeleteUser")
    .then((res) => {
      say("Good Bye! 👋");
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
  var body = { slackId: slack_id, moodName: mood };

  console.log(body);

  axios
    .post("/Record/AddRecord", body)
    .then((res) => {
      say("Thanks! 🤘");
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
  await app.start(process.env.PORT || 3000);
  job.start();
  console.log("🤖 Bot is running!");
})();
