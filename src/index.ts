import data from './mirrors.json';

for (let mirror of data.mirrors) {
    if (mirror.rsync) {
        let {host, src, dest, options} = mirror.rsync;
        console.log(`rsync ${options} ${host}:/${src} ${dest}`);
    }
}
