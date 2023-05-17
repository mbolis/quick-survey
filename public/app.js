"use strict"

const fieldTpl = document.querySelector(".field");
fieldTpl.remove();
fieldTpl.style.display = "";

async function render(el, surveyId) {
    el = document.querySelector(el);
    if (!el) throw new Error("root element not found");

    try {
        const resp = await fetch(`/api/surveys/${surveyId}`);
        if (resp.status !== 200) {
            throw new Error("survey not found");
        }

        const survey = await resp.json();

        const form = el.querySelector(".form");
        form.onsubmit = async function (e) {
            e.preventDefault();

            const submission = { fields: {} };
            for (const f of survey.fields || []) {
                const input = this.querySelector(`[name=${f.name}]`)
                let value;
                switch (f.type) {
                    case "text":
                    case "textarea":
                    case "select":
                        value = input.value;
                        break;
                    case "number":
                        value = +input.value;
                        break;
                    case "checkbox":
                        value = input.checked;
                        break;
                }
                submission.fields[f.name] = { id: f.id, value };
            }

            try {
                const resp = await fetch(`/api/surveys/${surveyId}/submissions`, {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(submission),
                });
                if (resp.status !== 201) {
                    throw new Error("could not send submission: " + await resp.text());
                }

                alert("Your submission was sent.");
                location.reload();

            } catch (err) {
                console.error(err);
                alert("There was an error!\n" + err.message);
            }
        };

        // TODO what if there was a way to show a message instead of the form, if a submission was already sent?

        el.querySelector(".title").textContent = survey.title || "";
        el.querySelector(".description").innerHTML = (survey.description || "") // XXX DON'T DO THIS!!!
            .replace(/\n\n/g, "<p>").replace(/\n/g, "<br>");

        const fieldsEl = el.querySelector(".fields");
        fieldsEl.innerHTML = "";
        for (const f of survey.fields || []) {
            const id = "field_" + f.name;

            const fieldEl = fieldTpl.cloneNode(true);
            if (f.required) {
                fieldEl.classList.add("required");
            }

            const label = fieldEl.querySelector("label");
            label.htmlFor = id;
            label.textContent = f.label;

            const fieldContainer = fieldEl.querySelector(".field-container");

            let input;
            switch (f.type) {
                case "text":
                    input = document.createElement("input");
                    input.id = id;
                    input.name = f.name;
                    input.required = f.required;
                    break;
                case "number":
                    input = document.createElement("input");
                    input.type = "number";
                    input.id = id;
                    input.name = f.name;
                    input.required = f.required;
                    break;
                case "checkbox":
                    input = document.createElement("input");
                    input.type = "checkbox";
                    input.id = id;
                    input.name = f.name;
                    input.required = f.required;
                    input.value = "1";
                    break;
                case "textarea":
                    input = document.createElement("textarea");
                    input.id = id;
                    input.name = f.name;
                    input.required = f.required;
                    break;
                case "select":
                    input = document.createElement("select");
                    input.id = id;
                    input.name = f.name;
                    input.required = f.required;

                    const emptyOption = document.createElement("option");
                    emptyOption.textContent = "--- Select one ---";
                    emptyOption.value = "";

                    input.append(emptyOption);

                    for (const o of f.options || []) {
                        const option = document.createElement("option");
                        option.textContent = o.label || "";
                        option.value = o.value;

                        input.append(option);
                    }
                    break;
            }

            fieldContainer.append(input);
            fieldsEl.append(fieldEl);
        }

        el.style.display = "block";

    } catch (err) {
        console.error(err);
        alert("There was an error!\n" + err.message);
    }

}