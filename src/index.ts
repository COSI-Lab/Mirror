import data from './mirrors.json';
import _ from './secrets.json';
let secrets = require('./secrets.json')["secrets"]
import fs from 'fs/promises'

async function createScripts() {
    await fs.mkdir("scripts/passwords", {recursive: true});

    for (let mirror of data.mirrors) {
        if (mirror.rsync) {
            let {host, src, dest, options} = mirror.rsync;
            if (secrets[mirror.short]) {
                let {username, password} = secrets[mirror.short];
                await fs.writeFile("scripts/passwords/" + mirror.short, password, {mode: "600"});
                let command = `#/bin/bash\nrsync ${options} --password-file=scripts/passwords/${mirror.short} ${username}@${host}::${src} ${dest}`;
                await fs.writeFile("scripts/" + mirror.short + ".sh", command, {mode: "755"});
            } else {
                let command = `#/bin/bash\nrsync ${options} ${host}::${src} ${dest}`;
                await fs.writeFile("scripts/" + mirror.short + ".sh", command, {mode: "755"});
            }
        }
    }
}

createScripts();