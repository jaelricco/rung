# Checkliste: Training loggen und einen neuen Skill lernen

Zwei Abläufe und die Schleife, die sie verbindet — in der Reihenfolge, in der ein
Athlet sie tatsächlich durchläuft. Alles hier ist an einer laufenden Instanz
durchgeklickt (Produktions-Build, echte Go-API, frische Postgres-Datenbank),
nicht aus dem Code abgelesen. Bildschirmtexte stehen wörtlich da, damit man sie
in der App wiederfindet.

---

## Einmalig: Konto

- [ ] **Konto anlegen.** Auf `/login` unten auf `Create one`, dann E-Mail, Name
      und ein Passwort mit mindestens 10 Zeichen. Falls oben `Continue with …`
      steht, geht auch Google oder ChatGPT — das ist reine Anmeldung und
      schaltet keine KI frei.
- [ ] **Optional: KI-Konto verbinden.** `Settings` → `AI connector` → Claude
      oder ChatGPT. Nötig nur für `Coach review` und `Sharpen it with AI`.
      Planer, Log, Leitern und Kalender laufen vollständig ohne — der Planer
      rechnet lokal in unter einer Sekunde.

---

## Ablauf A — Training loggen (nach jeder Einheit)

Das ist der Ablauf, der zählt: Level, Rekorde und jeder künftige Plan werden
ausschließlich aus geloggten Sätzen gerechnet.

1. [ ] Nach dem Training auf **`Log a session`** (linke Spalte, Gruppe
       `Training`; auf dem Handy erst das ☰-Menü).
2. [ ] **Übung wählen — das Formular richtet sich danach.** Ein Satz pro Zeile.
       Die Liste ist nach Kategorie gruppiert (`core · dynamic · legs ·
       mobility · pull · push · static · weighted`, 148 Übungen). Abgefragt
       wird je nach Übung `Reps`, `Reps` + `Added kg`, `Hold (s)` oder
       `Made / Missed`.
       *Die Liste öffnet immer auf `Arch body hold` und hat keine Suche — bei
       offenem Dropdown die ersten Buchstaben tippen ist schneller als
       scrollen.*
3. [ ] **Weitere Sätze mit `Add another set`.** Übernimmt Übung und Werte des
       letzten Satzes; drei gleiche Sätze sind drei Klicks. Beim Wechsel auf
       eine andersartige Übung bleibt das neue Feld leer — Sekunden und
       Wiederholungen sind nicht dasselbe.
4. [ ] **`Notes` und `Effort (1–10)`** ausfüllen. Beides optional, beides liest
       der Coach-Review später mit.
5. [ ] **`Save session`** — landet direkt auf der Übersicht mit
       `Level by category`, `Records`, `Last 28 days` und `Recent sessions`.
       ⚠️ **Kein Datumsfeld.** Jede Einheit wird auf den Moment des Speicherns
       gebucht; nachtragen geht in der Oberfläche nicht.
6. [ ] **Falls die Einheit im Kalender stand: separat abhaken.** Auf
       `Calendar` die Sitzung anklicken — sie öffnet in einem eigenen
       Browser-Tab unter `/session/…`, in den sechs Teilen, in denen sie
       ausgeführt wird: Joint Warm Up, Muscular Warm Up, Mobility, Specific
       Warm-Up, Training, Cooldown. Jeder Block hat ein Häkchen, oben läuft ein
       Zähler (`0 of 13 done`), und am Ende steht `Mark done`.
       ⚠️ **Zwei getrennte Handlungen.** `Mark done` hakt nur den Termin ab und
       schreibt keinen Satz mit — Level und Rekorde bewegen sich davon nicht.
       Umgekehrt hakt Loggen den Kalendereintrag nicht ab. Wer dem Plan folgt,
       macht beides.

---

## Ablauf B — Einen neuen Skill lernen (einmal pro Ziel)

Der Planer sortiert dich aus deinen Rekorden auf die Leiter zum Ziel ein.
Deshalb steht die Baseline vor dem Plan: ohne Zahlen startet jeder ganz unten.

1. [ ] **`Baseline` öffnen, die drei Kopfzahlen setzen.** `Bodyweight (kg)`
       macht aus Zusatzlast einen Prozentsatz, `Sessions a week` setzt das
       Startvolumen, `Sleep (hours)` steuert das Volumenwachstum (unter ~8 h
       geht der Plan vorsichtiger vor).
2. [ ] **Ausrüstung ankreuzen** unter `What you can train on`. Wirkt sofort: der
       getestete Plan schrieb selbst dazu, was er weggelassen hat („*Left out
       for what you do not: a way to add load (4 movements), gymnastic rings
       (2 movements) …*").
3. [ ] **Laufende Skills eintragen** unter `What you are learning right now`.
       Budget: 5 Einheiten pro Woche, Maximal kostet 3, Anspruchsvoll 2, alles
       andere 1. Zwei Maximalskills sprengen es bereits, und der Balken färbt
       sich rot.
4. [ ] **Ziel eintippen, `Ask about this goal` drücken.** Unten klappt
       `The ladder to …` auf (für „Front lever" sechs Sprossen von *Inverted
       hang* bis *Weighted front lever*).
5. [ ] **Kerntests und Leitersprossen ausfüllen.** `The eight that decide
       everything else` plus die Sprossen deiner Leiter. Leer lassen ist eine
       gültige Antwort — die erste Sprosse, die du *nicht* schaffst, ist
       ungefähr dein Startpunkt.
6. [ ] **`Save my baseline`**, dann dem Link `Build a plan from it` folgen. Der
       Link nimmt das Ziel mit.
7. [ ] **Auf `Plan` die Felder prüfen:** `Goal`, `Weeks` (1–24, Vorgabe 8),
       `Days per week` (1–7, Vorgabe 3), `Starts on` (Vorgabe: Montag dieser
       Woche). Ins Notizfeld gehört, was der Plan sonst nicht weiß.
8. [ ] **Entscheiden, wie viel Woche der Skill bekommt.** Drei Stufen, und die
       mittlere ist die Vorgabe:
       *Keep it in the week* (bis 20 %, eine Sitzung) — der langsamste Weg und
       der billigste, um alles andere zu behalten.
       *Train it properly* (bis 30 %, zwei Sitzungen) — was die Quellen für
       einen Skill vorsehen, den man wirklich verfolgt.
       *Build the week around it* (bis 40 %, drei Sitzungen) — andere Skills
       treten zurück. Mehr als 40 % gibt es nicht: darüber kauft man
       Gewebebelastung statt Fortschritt.
       `Sharpen it with AI` ist optional: ohne verbundenes KI-Konto bekommst du
       denselben Plan plus eine Zeile, die sagt, warum kein Modell drauf
       geschaut hat.
9. [ ] **`Generate plan`** und den Plan wirklich lesen: Titel nennt die Sprosse
       (*Front lever — 8 weeks · Inversion and body line*),
       `Where you are on the ladder` markiert mit ▶ und ✓, dann die Phasen
       (*Accumulation → Intensification → Test*) und die Kästen `Adjusted for` /
       `Trimmed before you saw it`. Eine Sitzung anklicken zeigt die Blöcke in
       Ausführungsreihenfolge — Prep, Skill, Kraft, Zubehör — mit Sätzen,
       Vorgabe, Pause und `Next week:` je Block.
       *In dieser Vorschau stehen technische Kürzel wie `scapular_pull_up`. Im
       Log-Dropdown und in der Sitzungsansicht nach dem Einplanen heißt dieselbe
       Übung ausgeschrieben.*
10. [ ] **`Add to my calendar`.** Erst dieser Klick macht aus dem Plan Termine
       (im Test 24 Sitzungen). Danach steht er unter `Plans on your calendar`
       mit Fortschrittszähler und `Remove`. `Download training.ics` bringt alles
       in die eigene Kalender-App.

---

## Die Schleife

Im Test verifiziert: 23 s Tuck Front Lever geloggt gegen einen Standard von
20 s — der nächste Plan startete daraufhin ohne weiteres Zutun eine Sprosse
höher.

1. **Die Sprosse trainieren** — was der Plan für diese Woche vorgibt.
2. **Ehrlich loggen** — vor allem die Zielbewegung. Ungeloggt bleibt der Plan
   bei deiner Baseline-Schätzung.
3. **Standard geknackt?** Die Leiter auf `Plan` zeigt, was als geklärt gilt.
4. **Neu generieren** — alten Plan im Kalender entfernen, neuen erzeugen und
   wieder eintragen.

Baseline nachziehen, sobald sich etwas Grundlegendes ändert (Ausrüstung,
Körpergewicht, zusätzlicher Skill im Budget). Alles echt Geloggte überschreibt
die Baseline-Zahl für diese Bewegung ohnehin automatisch.

---

## Optional

- **`Routine`** — die Woche, die du sowieso trainierst, einmal schreiben. Füllt
  den Kalender fortlaufend (`Every week, from that week on`) oder genau eine
  Woche (`Just that one week`).
- **`Review my training`** auf der Übersicht — liest die letzten vier Wochen.
  Braucht ein verbundenes KI-Konto; ohne kommt eine saubere Aufforderung mit
  Link, kein Fehler.
- **Sitzungen direkt in den Kalender schreiben** — auf einen leeren Tag klicken
  und die Einheit von Hand anlegen, in derselben Form wie eine Plansitzung.

---

## Beim Durchspielen gefunden

Vier Befunde, alle an einer laufenden Instanz reproduziert. Zwei davon hat
`main` inzwischen selbst behoben, in #31 und #36 — sie stehen hier, weil sie
erklären, warum zwei Schritte oben zeitweise ins Leere liefen. Die anderen
beiden waren auf `main` noch offen und sind hier behoben.

### Befund 1 — hier behoben: `Sharpen it with AI` schlägt für jeden Nutzer fehl

Die Planseite schickt das Ziel doppelt, als `skill` und als `goal`
(`frontend/src/routes/plan/+page.svelte`). `skillPlanRequest` in
`backend/internal/ai/plans.go` kennt nur `skill`, und `httpx.Decode` setzt
`DisallowUnknownFields()`. Ergebnis ist ein durchgereichter Go-Fehler:

```
400 That request body couldn't be read: json: unknown field "goal"
```

Das passiert, bevor geprüft wird, ob ein KI-Konto verbunden ist — der eigens
eingebaute Rückfall auf den algorithmischen Plan greift deshalb nie. Der
Kommentar über `SkillPlan` sagt ausdrücklich, dass dieser Endpunkt „no 428 and
no 502 any more" haben soll; tatsächlich antwortete er jedem mit 400.

**Fix:** `skillPlanRequest` nimmt beide Namen und löst sie über eine
`goal()`-Methode auf, mit derselben Präzedenz wie `plan.generateRequest`
(`goal` gewinnt). Beide Routen beantworten damit denselben Body gleich.
Verifiziert im Browser gegen den aktuellen `main`: 24 Sitzungen und die Zeile
*„Written by the app's own planner, because the model could not be used."*

### Befund 2 — hier behoben: Log-Formular im Vite-Dev-Server defekt

Unter `npm run dev` wirft jeder Wechsel im Übungs-Dropdown
`Cannot set properties of undefined (setting 'exercise_slug')`, und das Formular
bleibt beim ersten Eintrag stehen. Im Produktions-Build tritt das nicht auf.
Nutzer merken davon nichts; wer lokal entwickelt, schon.

Es ist auch nicht allgemein: `SessionEditor.svelte` hat dasselbe Muster und
funktioniert. Der Unterschied steht im kompilierten Output — `SessionEditor`
wird zu `$.each(…, 17, …)`, das Log-Formular zu `$.each(…, 21, …)`, also mit
Flag 4 (`EACH_IS_CONTROLLED`), weil der Block das einzige Kind seines
Containers ist. In dieser Kombination ist die Item-Quelle, die Svelte dem
Binding gibt, im Dev-Build bereits weg, wenn der Setter läuft.

**Fix:** Die Bindings schreiben über `sets[index]` statt über den each-Wert —
das trifft das Array-Signal, das immer lebt, und verhält sich im
Produktions-Build identisch. Der Item-Name heißt `_set`, damit niemand
versehentlich zurückrutscht. Verifiziert in beiden Builds über Hinzufügen,
Umstellen und Entfernen von Sätzen.

### Befund 3 — in #31 behoben: Skill-Katalog war bei jedem Start leer

`plan.SyncCatalogue` brach auf dem ersten Goal ab: 16 der 20 Goals setzen kein
`Feeds`, das nil-Slice ging als SQL-NULL an eine `not null`-Spalte, und die
Transaktion rollte zurück. `GET /api/v1/skills` antwortete dauerhaft leer, und
die Sehnenbudget-Auswahl auf `/baseline` hatte nichts anzuzeigen.

Behoben in `main` durch einen `list`-Helfer über allen Array-Parametern, plus
einem Projektionstest gegen eine echte Datenbank. #31 fand dabei noch einen
dritten Fehler, den dieses Durchspielen nicht sah: `BuildSnapshot` trug
`learning` gar nicht mit, sodass der Plan nach dem Picken wieder nur einen
Skill wog.

### Befund 4 — in #31/#36 behoben: Budget zählte in anderen Einheiten

14 Skills standen mit „· 0 units" da, obwohl ihre Überschrift „One unit each"
verspricht, und der Balken zählte sie mit null — während der Planer sie als 1
berechnete. `Goal.Units()` löst den Standard jetzt einmal dort auf, wo der Wert
geschrieben wird; Projektion und Endpunkt lesen darüber, und das Frontend
rechnet mit dem, was ankommt.

---

*Zuerst durchgespielt am 7. September 2026, gegen den aktuellen `main` erneut
geprüft am 10. September: Registrierung, Log, Baseline, Planerzeugung inklusive
Focus-Auswahl, Kalender, Sitzungsansicht und Abhaken, Routine, Events,
Einstellungen — Desktop und 390 px breit. Kein Layout lief seitlich über.*
