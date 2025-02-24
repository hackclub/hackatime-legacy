<p align="center">
  <img src="https://assets.hackclub.com/flag-orpheus-top.svg" width="350">

  <h1 align="center">Hackatime<h1>
</p>

<p align="center">
  <img src="https://badges.fw-web.space/github/license/hackclub/hackatime">
  <img src="https://waka.hackclub.com/api/badge/U062UG485EE/interval:any/project:hackatime?label=hackatime">
  <img src="https://badges.fw-web.space/github/languages/code-size/hackclub/hackatime">
  <a href="https://goreportcard.com/report/github.com/hackclub/hackatime">
  <img src="https://goreportcard.com/badge/github.com/hackclub/hackatime"></a>
</p>

<h3 align="center">A minimalist, self-hosted WakaTime-compatible backend for coding statistics.</h3>

<p align="center">
  <img src="static/assets/images/screenshot.png" width="500px">
</p>

Forked from [muety/wakapi](https://github.com/muety/wakapi) for a Hackclub specifc version. Huge thanks to [@muety](https://github.com/muety) for all the work he has done in [wakapi](https://github.com/muety/wakapi) which allowed us to spin this up so quickly :)

## ⌨️ How to use?

### 💻 Client setup

Hackatime relies on the open-source [WakaTime](https://github.com/wakatime/wakatime-cli) client tools. In order to collect
statistics for Hackatime, you need to set them up.

1. **Set up WakaTime** for your specific IDE or editor. Please refer to the
   respective [plugin guide](https://wakatime.com/plugins)
2. **Edit your local `~/.wakatime.cfg`** file as follows.

```ini
[settings]

# Your Hackatime server URL
api_url = http://localhost:3000/api

# Your Hackatime API key (get it from the web interface after having created an account)
api_key = 406fe41f-6d69-4183-a4cc-121e0c524c2b
```

Optionally, you can set up a [client-side proxy](https://github.com/muety/Hackatim/wiki/Advanced-Setup:-Client-side-proxy)
in addition.

### 🛠️ Server Setup

Clone the repo run `go build` and then `./hackatime -config config.yml`. More info available in [DOCS.md](DOCS.md).

## 🔧 API endpoints

See our [Swagger API Documentation](https://waka.hackclub.com/swagger-ui).

### Signup

For signing up user programaticaly you can use the `/signup` endpoint with the admin token as Bearer and it will return a json object similar to the following:

```ts
const signup = await fetch('http://localhost:8888/signup', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer blahaji_rulz_da_world'
  },
  body: new URLSearchParams({
    'location': 'America/New_York',
    'captcha_id': '',
    'invite_code': '',
    'username': 'test',
    'email': 'test@test.com',
    'password': '123456',
    'password_repeat': '123456'
  })
});

console.log(await signup.json())
```

```json
{"created":false,"api_key":"f91e9ae9-e667-44a6-bb1e-b40117e04439"}
```

If the user already exists then you will get a `true` value in the `created` field.

### Reset Password

For resetting a user's password programmatically, you can use the `/reset-password` endpoint with the admin token as Bearer and it will return a json object containing the reset token and user ID:

```ts
const resetPassword = await fetch('http://localhost:8888/reset-password', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer blahaji_rulz_da_world'
  },
  body: new URLSearchParams({
    'email': 'test@test.com'
  })
});

console.log(await resetPassword.json())
```

```json
{
  "user_id": "test",
  "reset_token": "d4e5f6-reset-token-g7h8i9"
}
```

If the user is not found, you will receive a 404 status code with an error message:

```json
{
  "error": "user not found"
}
```

The reset token can then be used with the `/set-password` endpoint to complete the password reset process.



### WakaTime integration

Hackatime plays well together with [WakaTime](https://wakatime.com). For one thing, you can **forward heartbeats** from
Hackatime to WakaTime to effectively use both services simultaneously. In addition, there is the option to **import
historic data** from WakaTime for consistency between both services. Both features can be enabled in the _Integrations_
section of your Hackatime instance's settings page.

### GitHub Readme Stats integrations

Hackatime also integrates
with [GitHub Readme Stats](https://github.com/anuraghazra/github-readme-stats#wakatime-week-stats) to generate fancy
cards for you. Here is an example. To use this, don't forget to **enable public data**
under [Settings -> Permissions](https://waka.hackclub.com/settings#permissions).

![](https://github-readme-stats.vercel.app/api/wakatime?username=U062UG485EE&api_domain=waka.hackclub.com&bg_color=2D3748&title_color=2F855A&icon_color=2F855A&text_color=ffffff&custom_title=Hackatime%20Week%20Stats&layout=compact&range=last_7_days)

<details>
<summary>Click to view code</summary>

```markdown
![](https://github-readme-stats.vercel.app/api/wakatime?username={yourusername}&api_domain=waka.hackclub.com&bg_color=2D3748&title_color=2F855A&icon_color=2F855A&text_color=ffffff&custom_title=Hackatime%20Week%20Stats&layout=compact)
```

</details>
<br>

### Github Readme Metrics integration

There is a [WakaTime plugin](https://github.com/lowlighter/metrics/tree/master/source/plugins/wakatime) for
GitHub [Metrics](https://github.com/lowlighter/metrics/) that is also compatible with Hackatime. To use this, don't forget
to **enable public data** under [Settings -> Permissions](https://waka.hackclub.com/settings#permissions).

Preview:

![](https://raw.githubusercontent.com/lowlighter/metrics/examples/metrics.plugin.wakatime.svg)

<details>
<summary>Click to view code</summary>

```yml
- uses: lowlighter/metrics@latest
  with:
      # ... other options
      plugin_wakatime: yes
      plugin_wakatime_token: ${{ secrets.WAKATIME_TOKEN }} # Required
      plugin_wakatime_days: 7 # Display last week stats
      plugin_wakatime_sections: time, projects, projects-graphs # Display time and projects sections, along with projects graphs
      plugin_wakatime_limit: 4 # Show 4 entries per graph
      plugin_wakatime_url: http://waka.hackclub.com # Wakatime url endpoint
      plugin_wakatime_user: .user.login # User
```

</details>
<br>

### Gnome Extension

If you're using the GNOME desktop, there is a quick way to display your today's coding statistics in the status bar.

![](.github/assets/screenshot_gnome.png)

Simply install the [Executor](https://extensions.gnome.org/extension/2932/executor/) extension and add the following
command as a status bar indicator:

```bash
~/.wakatime/wakatime-cli-linux-amd64 --today
```

## ❔ FAQs

Since Hackatime heavily relies on the concepts provided by WakaTime, [their FAQs](https://wakatime.com/faq) largely apply
to Hackatime as well. You might find answers there.

<details>
<summary><b>What data are sent to Hackatime?</b></summary>

<ul>
  <li>File names</li>
  <li>Project names</li>
  <li>Editor names</li>
  <li>Your computer's host name</li>
  <li>Timestamps for every action you take in your editor</li>
  <li>...</li>
</ul>

See the related [WakaTime FAQ section](https://wakatime.com/faq#data-collected) for details.

</details>

<details>
<summary><b>What happens if I'm offline?</b></summary>

All data are cached locally on your machine and sent in batches once you're online again.

</details>

<details>
<summary><b>How does Hackatime compare to WakaTime?</b></summary>

Hackatime is a small subset of WakaTime and has a lot less features. Cool WakaTime features, that are missing in Hackatime, include:

<ul>
  <li>Personal (non global) Leaderboards</li>
  <li><a href="https://wakatime.com/share/embed">Embeddable Charts</a></li>
  <li>Personal Goals</li>
  <li>Team / Organization Support</li>
  <li>Additional Integrations (with GitLab, etc.)</li>
  <li>Richer API</li>
</ul>

WakaTime is worth the price. However, this is made to be open source and free!!!

</details>

<details>
<summary><b>How are durations calculated?</b></summary>

Inferring a measure for your coding time from heartbeats works a bit differently than in WakaTime. While WakaTime
has <a href="https://wakatime.com/faq#timeout">timeout intervals</a>, Hackatime essentially just pads every heartbeat that
occurs after a longer pause with 2 extra minutes.

Here is an example (circles are heartbeats):

```text
|---o---o--------------o---o---|
|   |10s|      3m      |10s|   |

```

It is unclear how to handle the three minutes in between. Did the developer do a 3-minute break, or were just no
heartbeats being sent, e.g. because the developer was staring at the screen trying to find a solution, but not actually
typing code?

<ul>
  <li><b>WakaTime</b> (with 5 min timeout): 3 min 20 sec
  <li><b>WakaTime</b> (with 2 min timeout): 20 sec
  <li><b>Hackatime:</b> 10 sec + 2 min + 10 sec = 2 min 20 sec</li>
</ul>

Hackatime adds a "padding" of two minutes before the third heartbeat. This is why total times will slightly vary between
Hackatime and WakaTime.

</details>

## 🙏 Thanks

Thanks a ton to [muety/wakapi](https://github.com/muety/wakapi) for making this project and for making it opensource and under a permissable license!

## 📓 License

MIT @ [Kieran Klukas](https://github.com/kcoderhtml)
