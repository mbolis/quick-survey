"use strict"

// XXX copy-pasta'd from /admin
const fieldTpl = document.querySelector(".fields-item");
fieldTpl.remove();
fieldTpl.style.display = "";

const editorForm = document.querySelector("#editor")
const ul = document.querySelector("#fields");

const ONE_YEAR = 60 * 60 * 24 * 365; // XXX who's this guy here?

startup();
async function startup() {
  // XXX copy-pasta'd from /login
  const cookies = Object.fromEntries(document.cookie
    .split(/\s*;\s*/)
    .map(c => {
      const ieq = c.indexOf("=");
      return [c.slice(0, ieq), c.slice(ieq + 1)];
    }));

  const isNew = !!~location.search.indexOf("new");
  const surveyId = ~~!isNew && +location.search
    .replace(/^\?/, "")
    .split("&")
    .find(x => x.match(/^id=/))
    ?.split("=")[1]

  try {
    let survey;
    if (isNew) {
      document.querySelector("#main_title").textContent = "Edit new survey";

      survey = {
        title: "",
        description: "",
        fields: [],
      };

    } else {
      document.querySelector("#main_title").textContent = "Loading survey #" + surveyId;

      const resp = await fetch(`/api/admin/surveys/${surveyId}`, {
        headers: {
          // XXX this is all good when the page gets loaded, but... see below...
          Authorization: "Bearer " + cookies.access_token,
        },
      });
      if (resp.status !== 200) {
        throw new Error("could not retrieve survey: " + await resp.text());
      }

      survey = await resp.json();
      document.querySelector("#main_title").textContent = "Edit survey #" + surveyId;
    }

    editorForm.style.display = "block";

    editorForm.onsubmit = async e => {
      e.preventDefault();

      // XXX copy-pasta galore
      const cookies = Object.fromEntries(document.cookie
        .split(/\s*;\s*/)
        .map(c => {
          const ieq = c.indexOf("=");
          return [c.slice(0, ieq), c.slice(ieq + 1)];
        }));

      try {
        if (!cookies.access_token) {
          const resp = await fetch("/api/refresh", {
            method: "POST",
            headers: {
              Authorization: "Refresh " + cookies.refresh_token,
            },
          });
          if (resp.status !== 200) {
            throw new Error("could not refresh token: " + await resp.text());
          }

          // XXX more copy-pasta
          const { access_token, refresh_token, expires_in } = await resp.json();
          document.cookie = `access_token=${access_token};max-age=${expires_in};path=/`;
          document.cookie = `refresh_token=${refresh_token};max-age=${ONE_YEAR};path=/`;

          cookies.access_token = access_token;
          cookies.refresh_token = refresh_token;
        }

        let resp = isNew
          ? await fetch("/api/admin/surveys", {
            method: "POST",
            headers: {
              Authorization: "Bearer " + cookies.access_token,
              "Content-Type": "application/json",
            },
            body: JSON.stringify(survey),
          })
          : await fetch(`/api/admin/surveys/${surveyId}`, {
            method: "PUT",
            headers: {
              Authorization: "Bearer " + cookies.access_token,
              "Content-Type": "application/json",
            },
            body: JSON.stringify(survey),
          });
        if (resp.status === 401) {
          // XXX do the same thing as before... copy-pasta extravaganza!
          resp = await fetch("/api/refresh", {
            method: "POST",
            headers: {
              Authorization: "Refresh " + cookies.refresh_token,
            },
          });
          if (resp.status !== 200) {
            throw new Error("could not refresh token: " + await resp.text());
          }

          const { access_token, refresh_token, expires_in } = await resp.json();
          document.cookie = `access_token=${access_token};max-age=${expires_in};path=/`;
          document.cookie = `refresh_token=${refresh_token};max-age=${ONE_YEAR};path=/`;

          cookies.access_token = access_token;
          cookies.refresh_token = refresh_token;

          // XXX copy-pasta inception!
          resp = isNew
            ? await fetch("/api/admin/surveys", {
              method: "POST",
              headers: {
                Authorization: "Bearer " + cookies.access_token,
                "Content-Type": "application/json",
              },
              body: JSON.stringify(survey),
            })
            : await fetch(`/api/admin/surveys/${surveyId}`, {
              method: "PUT",
              headers: {
                Authorization: "Bearer " + cookies.access_token,
                "Content-Type": "application/json",
              },
              body: JSON.stringify(survey),
            });
        }
        if (resp.status === 204) {
          window.location.reload();
        } else if (resp.status === 201) {
          const { id } = await resp.json();
          window.location = "/admin/edit?id=" + id;
        } else {
          throw new Error("could not save survey: " + await resp.text());
        }
      } catch (err) {
        console.error(err);
        alert("There was an error!\n" + err.message);
      }
    };

    Object.assign(editorForm.querySelector("#title"), {
      value: survey.title,
      oninput() {
        survey.title = this.value;
      },
    });

    Object.assign(editorForm.querySelector("#description"), {
      value: survey.description,
      oninput() {
        survey.description = this.value;
      },
    });

    for (const f of survey.fields) {
      const li = fieldTpl.cloneNode(true);

      const label = li.querySelector(".label");
      label.querySelector("label").htmlFor = "field_" + f.id + "_label";
      Object.assign(label.querySelector("input"), {
        id: "field_" + f.id + "_label",
        value: f.label,
        oninput() {
          f.label = this.value;
        },
      });

      const type = li.querySelector(".type");
      type.querySelector("label").htmlFor = "field_" + f.id + "_type";
      Object.assign(type.querySelector("select"), {
        id: "field_" + f.id + "_type",
        value: f.type,
        onchange() {
          f.type = this.value;

          const options = li.querySelector(".options");
          const optionsInput = options.querySelector("textarea");
          if (this.value === "select") {
            options.style.display = "block";
            optionsInput.value = "[]";
          } else {
            options.style.display = "none";
            optionsInput.value = "";
          }
          optionsInput.oninput();
        },
      });

      const options = li.querySelector(".options");
      options.querySelector("textarea").oninput = function () {
        if (/^\s*$/.test(this.value)) {
          f.options = null;
        } else {
          try {
            f.options = JSON.parse(this.value); // XXX there was a bug here, very tricky to fix because of code duplication!
          } catch (err) {
            f.options = err;
          }
        }
      };
      if (f.options) {
        options.style.display = "block";
        options.querySelector("textarea").value = JSON.stringify(f.options, null, 2);
      }

      const required = li.querySelector(".required");
      required.querySelector("label").htmlFor = "field_" + f.id + "_required";
      Object.assign(required.querySelector("input"), {
        id: "field_" + f.id + "_required",
        checked: f.required,
        onchange() {
          f.required = this.checked;
        },
      });

      li.querySelector(".up").onclick = () => {
        if (ul.firstElementChild === li) return;
        li.previousSibling.insertAdjacentElement("beforebegin", li);

        const idx = survey.fields.indexOf(f);
        survey.fields.splice(idx - 1, 0, ...survey.fields.splice(idx, 1));
      };
      li.querySelector(".down").onclick = () => {
        if (ul.lastElementChild === li) return;
        li.nextSibling.insertAdjacentElement("afterend", li);

        const idx = survey.fields.indexOf(f);
        survey.fields.splice(idx + 1, 0, ...survey.fields.splice(idx, 1));
      };
      li.querySelector(".remove").onclick = () => {
        li.remove();
        survey.fields.splice(survey.fields.indexOf(f), 1);
      };

      ul.append(li);
    }

    let newFieldId = -1;
    document.querySelector("#add").onclick = () => {
      // XXX here comes some mean copy-pasta
      const li = fieldTpl.cloneNode(true);

      const id = newFieldId--;
      const f = { id, label: "", type: "text", required: false };

      const label = li.querySelector(".label");
      label.querySelector("label").htmlFor = "field_" + id + "_label";
      Object.assign(label.querySelector("input"), {
        id: "field_" + id + "_label",
        oninput() {
          f.label = this.value;
        },
      });

      const type = li.querySelector(".type");
      type.querySelector("label").htmlFor = "field_" + id + "_type";
      Object.assign(type.querySelector("select"), {
        id: "field_" + id + "_type",
        onchange() {
          f.type = this.value;

          const options = li.querySelector(".options");
          const optionsInput = options.querySelector("textarea");
          if (this.value === "select") {
            options.style.display = "block";
            optionsInput.value = "[]";
          } else {
            options.style.display = "none";
            optionsInput.value = "";
          }
          optionsInput.oninput();
        },
      });

      li.querySelector(".options textarea").oninput = function () {
        if (/^\s*$/.test(this.value)) {
          f.options = null;
        } else {
          try {
            f.options = JSON.parse(this.value);
          } catch (err) {
            f.options = err; // TODO handle error
          }
        }
      };

      const required = li.querySelector(".required");
      required.querySelector("label").htmlFor = "field_" + id + "_required";
      Object.assign(label.querySelector("input"), {
        id: "field_" + id + "_label",
        oninput() {
          f.required = this.checked;
        },
      });

      li.querySelector(".up").onclick = () => {
        if (ul.firstElementChild === li) return;
        li.previousSibling.insertAdjacentElement("beforebegin", li);

        const idx = survey.fields.indexOf(f);
        survey.fields.splice(idx - 1, 0, ...survey.fields.splice(idx, 1));
      };
      li.querySelector(".down").onclick = () => {
        if (ul.lastElementChild === li) return;
        li.nextSibling.insertAdjacentElement("afterend", li);

        const idx = survey.fields.indexOf(f);
        survey.fields.splice(idx + 1, 0, ...survey.fields.splice(idx, 1));
      };
      li.querySelector(".remove").onclick = () => {
        li.remove();
        survey.fields.splice(survey.fields.indexOf(f), 1);
      };

      survey.fields.push(f);
      ul.append(li);
    };

    if (surveyId) {
      const deleteButton = document.querySelector("#delete");
      deleteButton.style.display = "";
      deleteButton.onclick = async () => {
        // XXX ultimate copy-pasta
        const cookies = Object.fromEntries(document.cookie
          .split(/\s*;\s*/)
          .map(c => {
            const ieq = c.indexOf("=");
            return [c.slice(0, ieq), c.slice(ieq + 1)];
          }));

        try {
          if (!cookies.access_token) {
            const resp = await fetch("/api/refresh", {
              method: "POST",
              headers: {
                Authorization: "Refresh " + cookies.refresh_token,
              },
            });
            if (resp.status !== 200) {
              throw new Error("could not refresh token: " + await resp.text());
            }

            const { access_token, refresh_token, expires_in } = await resp.json();
            document.cookie = `access_token=${access_token};max-age=${expires_in};path=/`;
            document.cookie = `refresh_token=${refresh_token};max-age=${ONE_YEAR};path=/`;

            cookies.access_token = access_token;
            cookies.refresh_token = refresh_token;
          }

          // XXX this is the only changing bit
          let resp = await fetch(`/api/admin/surveys/${surveyId}`, {
            method: "DELETE",
            headers: {
              Authorization: "Bearer " + cookies.access_token,
            },
          });
          //

          if (resp.status === 401) {
            resp = await fetch("/api/refresh", {
              method: "POST",
              headers: {
                Authorization: "Refresh " + cookies.refresh_token,
              },
            });
            if (resp.status !== 200) {
              throw new Error("could not refresh token: " + await resp.text());
            }

            const { access_token, refresh_token, expires_in } = await resp.json();
            document.cookie = `access_token=${access_token};max-age=${expires_in};path=/`;
            document.cookie = `refresh_token=${refresh_token};max-age=${ONE_YEAR};path=/`;

            cookies.access_token = access_token;
            cookies.refresh_token = refresh_token;

            // XXX
            resp = resp = await fetch(`/api/admin/surveys/${surveyId}`, {
              method: "DELETE",
              headers: {
                Authorization: "Bearer " + cookies.access_token,
              },
            });
            //
          }
          // XXX this too is different...
          if (resp.status !== 204) {
            throw new Error("could not save survey: " + await resp.text());
          }
          window.location = "/admin";
          //

        } catch (err) {
          console.error(err);
          alert("There was an error!\n" + err.message);
        }
      };
    }

  } catch (err) {
    console.error(err);
    alert("There was an error!\n" + err.message);
  }
}