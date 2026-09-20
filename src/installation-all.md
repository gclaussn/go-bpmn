---
description: A guide on how to install daemon and CLI.
---

<script setup>
import release from './assets/release.json'
</script>

# Installation

Supported operating systems and architectures:

<ul v-for="artifact in release.artifacts">
  <li><a :href='"installation-" + artifact.osarch'>{{ artifact.osarch }}</a></li>
</ul>

Looking for a specific version? - [View releases on Github](https://github.com/gclaussn/go-bpmn/releases)
