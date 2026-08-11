<h1 align="center">Telegram File Stream Bot</h1>
<p align="center">
  </a>
  <p align="center">
    <a herf="https://github.com/EverythingSuckz/TG-FileStreamBot">
        <img src="https://filestream.bot/logo.svg" height="100" width="100" alt="File Stream Bot Logo">
    </a>
</p>
  <p align="center">
    A Telegram bot to <b>generate direct link</b> for your Telegram files.
    <br />
    << <a href="https://filestream.bot/getting-started">Visit docs</a> |  <a href="https://filestream.bot/support">Support</a> >>
  </p>
</p>

## Safe local setup

This fork builds and runs as a separate service. It does not require an external
database: the Telegram session is stored in a local SQLite session file and file
metadata is cached in memory.

Create the ignored local configuration without adding secrets to Git:

```sh
cp fsb.sample.env fsb.env
chmod 600 fsb.env
```

Fill `API_ID`, `API_HASH`, `BOT_TOKEN`, `LOG_CHANNEL`, `ALLOWED_USERS`, and
`HOST` locally. `ALLOWED_USERS` is a comma-separated list of numeric Telegram
user IDs, for example `12345,67890` (no usernames). Leaving it empty allows any
Telegram user who can contact the bot to submit files. It does not restrict
people who already possess a generated HTTP link.

Generated links are bearer links authenticated with an HMAC derived from the bot
token. Keep `HASH_LENGTH` between 24 and 32 and do not publish complete URLs. The
HTTP logger omits query strings so hashes are not written to access logs. For a
local-only first test, use `HOST=http://localhost:8080`.

Build the fork without starting the bot:

```sh
docker compose build
```




## Credits

- [@celestix](https://github.com/celestix) for [gotgproto](https://github.com/celestix/gotgproto)
- [@divyam234](https://github.com/divyam234/teldrive) for his [Teldrive](https://github.com/divyam234/teldrive) Project
- [@krau](https://github.com/krau) for adding image support

## Copyright

Copyright (C) 2026 [EverythingSuckz](https://github.com/EverythingSuckz) under [GNU Affero General Public License](https://www.gnu.org/licenses/agpl-3.0.en.html).

TG-FileStreamBot is Free Software: You can use, study share and improve it at your
will. Specifically you can redistribute and/or modify it under the terms of the
[GNU Affero General Public License](https://www.gnu.org/licenses/agpl-3.0.en.html) as
published by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version. Also keep in mind that all the forks of this repository MUST BE OPEN-SOURCE and MUST BE UNDER THE SAME LICENSE.
