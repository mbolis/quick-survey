"use strict"

const fieldTpl = document.querySelector(".surveys-item");
fieldTpl.remove();
fieldTpl.style.display = "block";

const ul = document.querySelector("#surveys");

startup();
async function startup() {
    // XXX copy-pasta'd from /login
    const cookies = Object.fromEntries(document.cookie
        .split(/\s*;\s*/) // XXX there was a bug due to a bad regex here... had to update it everywhere!
        .map(c => {
            const ieq = c.indexOf("=");
            return [c.slice(0, ieq), c.slice(ieq + 1)];
        })); // XXX same thing here...
    try {
        const resp = await fetch("/api/admin/surveys", {
            headers: {
                Authorization: "Bearer " + cookies.access_token,
            },
        });
        if (resp.status !== 200) {
            throw new Error("could not retrieve surveys: " + await resp.text());
        }

        const { surveys } = await resp.json();
        for (const s of surveys) {
            const li = fieldTpl.cloneNode(true);

            li.querySelector(".title").textContent = `#${s.id}: ${s.title}`;
            li.querySelector(".description").innerHTML = s.description || "";
            li.querySelector(".edit").href = "/admin/edit?id=" + s.id;
            li.querySelector(".submissions").href = "/admin/submissions?id=" + s.id;

            ul.append(li);
        }

        document.querySelector("#add").onclick = () => {
            window.location = "/admin/edit?new";
        };

    } catch (err) {
        console.error(err);
        alert("There was an error!\n" + err.message);
    }
}