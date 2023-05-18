"use strict"

const headRow = document.querySelector(".head-row");
const fieldHeaderTpl = headRow.querySelector(".field");
fieldHeaderTpl.remove();
fieldHeaderTpl.style.display = "";

const tbody = document.querySelector("#submissions tbody");

const submissionRowTpl = document.querySelector(".submissions-row");
submissionRowTpl.remove();
submissionRowTpl.style.display = "";

const submissionFieldTpl = submissionRowTpl.querySelector(".field");
submissionFieldTpl.remove();

const viz = {
  active: false,
  fields: {},
  data: [],
};

const vizRowTpl = document.querySelector(".viz-row");
vizRowTpl.remove();

startup();
// XXX the juicy fruit of copy-pasta
async function startup() {
  const cookies = Object.fromEntries(document.cookie
    .split(/\s*;\s*/)
    .map(c => {
      const ieq = c.indexOf("=");
      return [c.slice(0, ieq), c.slice(ieq + 1)];
    }));

  const surveyId = +location.search
    .replace(/^\?/, "")
    .split("&")
    .find(x => x.match(/^id=/))
    ?.split("=")[1]

  try {
    let resp = await fetch(`/api/admin/surveys/${surveyId}`, {
      headers: {
        Authorization: "Bearer " + cookies.access_token,
      },
    });
    if (resp.status !== 200) {
      throw new Error("could not retrieve survey: " + await resp.text());
    }

    const survey = await resp.json();
    document.querySelector(".title").textContent = survey.title || "";
    document.querySelector(".description").innerHTML = survey.description || "";

    for (const f of survey.fields || []) {
      const th = fieldHeaderTpl.cloneNode(true);

      th.querySelector(".field-label").textContent = f.label;

      const vizMode = th.querySelector(".field-viz-mode");
      vizMode.onchange = function () {
        viz.fields[f.name] = vizMode.value;
      };
      th.querySelector(".field-viz").onchange = function () {
        if (this.checked) {
          vizMode.style.display = "";
          viz.fields[f.name] = vizMode.value;
        } else {
          vizMode.style.display = "none";
          viz.fields[f.name] = null;
        }
        
        if (!!Object.values(viz.fields).filter(x => x).length) {
          document.querySelector("#viz").disabled = false;
        } else {
          document.querySelector("#viz").disabled = true;
        }
      };

      headRow.append(th);
    }

    resp = await fetch(`/api/admin/surveys/${surveyId}/submissions`, {
      headers: {
        Authorization: "Bearer " + cookies.access_token,
      },
    });
    if (resp.status !== 200) {
      throw new Error("could not retrieve submissions: " + await resp.text());
    }

    const { submissions } = await resp.json();
    for (const s of submissions) {
      const tr = submissionRowTpl.cloneNode(true);

      tr.querySelector(".id").textContent = s.id;

      const data = {};
      for (const f of Object.values(s.fields) || []) {
        const td = submissionFieldTpl.cloneNode(true);
        td.textContent = f.value;
        tr.append(td);

        data[f.name] = f.value;
      }
      viz.data.push(data);

      tbody.append(tr);
    }

    Object.assign(document.querySelector("#viz"), {
      disabled: true,
      onclick() {
        const table = document.querySelector("#submissions");
        const vizBox = document.querySelector("#viz_box");
        vizBox.innerHTML = "";

        if (viz.active) {
          viz.active = false;
          this.textContent = "VIZ!";
          table.parentElement.style.display = "";
          vizBox.style.display = "none";
          
        } else {
          viz.active = true;
          this.textContent = "<- Data";
          table.parentElement.style.display = "none";
          vizBox.style.display = "";

          // viz whiz!!!
          for (const f of survey.fields || []) {
            if (!viz.fields[f.name]) continue;

            // extract field values
            const aggregateData = viz.data
              .map(r => r[f.name])
              .map(v => {
                switch (f.type) {
                  case "checkbox":
                    return v ? "yes" : "no";
                  case "text":
                    return v.toLowerCase();
                  default:
                    return v;
                }
              })
              // aggregate values
              .reduce((aggr, value) => {
                if (viz.fields[f.name] === "tag-cloud") {
                  // prepare tag cloud
                  const words = value
                    // - split words
                    .split(/\s+/g)
                    // - lowercase
                    .map(w => w.toLowerCase().replace(/[!-#%-*,-/:;?@\[-\]_{}\xa1\xa7\xab\xb6\xb7\xbb\xbf\u037e\u0387\u055a-\u055f\u0589\u058a\u05be\u05c0\u05c3\u05c6\u05f3\u05f4\u0609\u060a\u060c\u060d\u061b\u061e\u061f\u066a-\u066d\u06d4\u0700-\u070d\u07f7-\u07f9\u0830-\u083e\u085e\u0964\u0965\u0970\u0af0\u0df4\u0e4f\u0e5a\u0e5b\u0f04-\u0f12\u0f14\u0f3a-\u0f3d\u0f85\u0fd0-\u0fd4\u0fd9\u0fda\u104a-\u104f\u10fb\u1360-\u1368\u1400\u166d\u166e\u169b\u169c\u16eb-\u16ed\u1735\u1736\u17d4-\u17d6\u17d8-\u17da\u1800-\u180a\u1944\u1945\u1a1e\u1a1f\u1aa0-\u1aa6\u1aa8-\u1aad\u1b5a-\u1b60\u1bfc-\u1bff\u1c3b-\u1c3f\u1c7e\u1c7f\u1cc0-\u1cc7\u1cd3\u2010-\u2027\u2030-\u2043\u2045-\u2051\u2053-\u205e\u207d\u207e\u208d\u208e\u2329\u232a\u2768-\u2775\u27c5\u27c6\u27e6-\u27ef\u2983-\u2998\u29d8-\u29db\u29fc\u29fd\u2cf9-\u2cfc\u2cfe\u2cff\u2d70\u2e00-\u2e2e\u2e30-\u2e3b\u3001-\u3003\u3008-\u3011\u3014-\u301f\u3030\u303d\u30a0\u30fb\ua4fe\ua4ff\ua60d-\ua60f\ua673\ua67e\ua6f2-\ua6f7\ua874-\ua877\ua8ce\ua8cf\ua8f8-\ua8fa\ua92e\ua92f\ua95f\ua9c1-\ua9cd\ua9de\ua9df\uaa5c-\uaa5f\uaade\uaadf\uaaf0\uaaf1\uabeb\ufd3e\ufd3f\ufe10-\ufe19\ufe30-\ufe52\ufe54-\ufe61\ufe63\ufe68\ufe6a\ufe6b\uff01-\uff03\uff05-\uff0a\uff0c-\uff0f\uff1a\uff1b\uff1f\uff20\uff3b-\uff3d\uff3f\uff5b\uff5d\uff5f-\uff65]/gu, ""))
                    // - filter stop-words
                    .filter(w => !/^(@|https?:|\/\/)/.test(w) && !Stopwords.it.includes(w) && !Stopwords.en.includes(w));

                  const counts = {};
                  // - deduplicate - boost repeated words a bit
                  for (const w of words) {
                    if (w in counts) {
                      counts[w] += 0.1; // XXX boost factor is a magic number
                    } else {
                      counts[w] = 1;
                    }
                  }
                  // - update aggregate count
                  for (const [w, c] of Object.entries(counts)) {
                    if (c) {
                      aggr[w] = (aggr[w] || 0) + c;
                    }
                  }
                } else {
                  aggr[value] = (aggr[value] || 0) + 1;
                }
                return aggr;
              }, {});

            const dataset = [];
            switch (f.type) {
              case "select":
                dataset.push(...f.options.map(o => ({ label: o.label, value: aggregateData[o.value] })));
                break;
              case "checkbox":
                dataset.push({ label: "Yes", value: aggregateData.yes }, { label: "No", value: aggregateData.no });
                break;
              case "number":
                dataset.push(...Object.entries(aggregateData)
                  .sort(([v1], [v2]) => +v1 - v2)
                  .map(([label, value]) => ({ label, value })));
                break;
              default:
                dataset.push(...Object.entries(aggregateData)
                  .sort(([v1], [v2]) => v1.localeCompare(v2))
                  .map(([label, value]) => ({ label, value })));
            }

            // viz aggregated values
            const row = vizRowTpl.cloneNode(true);
            row.querySelector(".label").textContent = f.label;
            vizBox.append(row);

            const answer = row.querySelector(".answer");
            const width = answer.offsetWidth;

            switch (viz.fields[f.name]) {
              case "histogram": {
                const canvas = document.createElement("canvas");
                answer.append(canvas);
                console.log(dataset)
                new Chart(canvas, {
                  type: "bar",
                  data: {
                    labels: dataset.map(r => r.label),
                    datasets: [{ label: f.label, data: dataset.map(r => r.value) }],
                  },
                });
                break;
              }
              case "pie": {
                const canvas = document.createElement("canvas");
                new Chart(canvas, {
                  type: "pie",
                  data: {
                    labels: dataset.map(r => r.label),
                    datasets: [{ data: dataset.map(r => r.value) }],
                  },
                });
                answer.append(canvas);
                break;
              }
              case "tag-cloud": {
                const [min, max] = dataset.map(r => r.value).reduce(
                  ([min, max], v) => [Math.min(min, v - 1), Math.max(max, v)],
                  [+Infinity, -Infinity],
                );
                const delta = max - min;
                const words = dataset.map(r => ({
                  text: r.label,
                  size: Math.round(10 + 90 * (r.value - min) / delta), // XXX a mysterious formula with fudgy magic numbers
                }));
                console.log(words)
                const size = width < 400 ? [width, 600] : [width, 400];// XXX more magic numbers
                d3.layout.cloud()
                  .size(size)
                  .words(words)
                  .padding(5)
                  .rotate(() => 0)
                  .font("Roboto, sans-serif")
                  .fontSize(d => d.size)
                  .on("end", function draw(words) {
                    d3.select(answer)
                      .append("svg").attr("width", size[0]).attr("height", size[1])
                      .append("g").attr("transform", "translate(" + size[0] / 2 + "," + size[1] / 2 + ")")
                      .selectAll("text").data(words)
                      .enter()
                    /**/.append("text")
                    /**/.style("font-size", d => d.size + "px")
                    /**/.style("font-family", "Roboto, sans-serif")
                    /**/.attr("text-anchor", "middle")
                    /**/.attr("transform", d => "translate(" + [d.x, d.y] + ")rotate(" + d.rotate + ")")
                    /**/.attr("fill", randomColor)
                    /**/.text(d => d.text);
                  })
                  .start();
              }
            }
          }
          // XXX end of a ~130 lines else-branch...
        }
      }
    });

  } catch (err) {
    console.error(err);
    alert("There was an error!\n" + err.message);
  }
}