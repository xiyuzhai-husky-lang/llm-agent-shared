# Correspondence

A correspondence is a web page built for communicating with the user remotely, over Telegram or a similar channel. Every page belongs to a class, and a class is always named `<genre name>/<class name>`: the genre is the kind of page, the class is one concrete form of that kind, such as `diagnosis/basic` or `explanation/testing-situation`.

Two directory trees are laid out by class:

- `classes/<genre>/<class>/`: the definition of the class. The guideline fixes content and layout; `ui/build.sh` builds the page.
- `examples/<genre>/<class>/<example-name>/`: typical examples, named after their content, never after a date or serial.

## Writing

A page separates the main content from the secondary. It opens with the main content, such as the rule a plan proposes; secondary details, such as what an addition was modelled on, come after it.
