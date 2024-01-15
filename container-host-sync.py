import asyncio
import aiodocker
import json
import os
import subprocess


async def list_container():
    container = await docker.containers.list()
    entries = []
    for c in container:
      a = await c.show()
      entry = a["NetworkSettings"]["Networks"]["overlay"]["IPAddress"] + "   " + a["Name"][1:] + "\n"
      print(entry)
      entries.append(entry)

    with open('/etc/dnsmasq.hosts', 'w') as hosts:
      hosts.truncate(0)
      hosts.writelines(entries)

    subprocess.run(["systemctl", "reload", "dnsmasq"])

    await docker.close()


if __name__ == '__main__':
    docker = aiodocker.Docker()
    loop = asyncio.get_event_loop()
    loop.run_until_complete(list_container())
    loop.close()