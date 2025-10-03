# Docs

[Astro Starlight](https://starlight.astro.build/) based documentation.

## Develop

To be able to develop, the information about a go-bpmn release must be fetched first.

Specify the release tag:

```sh
export RELEASE_TAG_NAME="vX.Y.Z"
```

Run the following command to put a `release.json` file under `src/assets/`:

```sh
curl -L \
--create-dirs -o src/assets/release.json \
https://github.com/gclaussn/go-bpmn/releases/download/${RELEASE_TAG_NAME}/release.json
```

Start the development server:

```sh
npm run dev
```

## Build

```sh
bash ./.github/workflows/build.sh ${RELEASE_TAG_NAME}
```

## Preview

To preview a built documentation, run:

```sh
npm run preview
```
