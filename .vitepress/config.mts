import { defineConfig, DefaultTheme } from 'vitepress'

import release from '../src/assets/release.json'

const githubUrl = 'https://github.com/gclaussn/go-bpmn'
const code = preprocessCode()

// https://vitepress.dev/reference/site-config
export default defineConfig({
  srcDir: './src',

  base: '/go-bpmn/',

  title: 'go-bpmn',
  description: 'A native BPMN 2.0 process engine, built on top of PostgreSQL.',

  head: [
    ['link', { rel: 'icon', href: '/go-bpmn/favicon.ico' }]
  ],

  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    nav: [
      {
        text: 'Home',
        link: '/',
      },
      {
        text: 'Docs',
        link: '/introduction',
      },
    ],

    sidebar: {
      '/': { base: '/', items: sidebar() },
    },

    socialLinks: [
      { icon: 'github', link: githubUrl }
    ],

    footer: {
      message: 'Released under the MIT License.',
      copyright: '<a href="https://github.com/egonelbre/gophers" target="_blank">Gopher icons</a> by @egonelbre'
    }
  },

  markdown: {
    config: (md) => {
      const defaultRender = md.renderer.rules.fence
      if (!defaultRender) {
        return
      }

      md.renderer.rules.fence = (tokens, idx, options, env, self) => {
        const token = tokens[idx]

        if (token.info.startsWith('preprocess')) {
          const codeKey = token.content.trim() // remove trailing newline

          token.content = code[codeKey]

          const info = token.info.split(' ') // e.g. 'preprocess sh [Linux]' -> ['preprocess', 'sh', '[Linux]']
          info.shift() // e.g. ['preprocess', 'sh'] -> ['sh', '[Linux]']
          token.info = info.join(' ') // e.g. ['sh', '[Linux]'] -> 'sh [Linux]'
        }

        return defaultRender(tokens, idx, options, env, self)
      }
    }
  }
})

function sidebar(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: 'Getting started',
      collapsed: false,
      items: [
        { text: 'Introduction', link: 'introduction' },
        { text: 'Automate a process', link: 'automate-process' },
      ]
    },
    {
      text: 'Guides',
      collapsed: false,
      items: [
        {
          text: 'Installation',
          link: 'all',
          base: 'installation-',
          collapsed: true,
          items: [
            { text: 'darwin-arm64', link: 'darwin-arm64' },
            { text: 'linux-amd64', link: 'linux-amd64' },
            { text: 'linux-arm64', link: 'linux-arm64' },
            { text: 'windows-amd64', link: 'windows-amd64' },
          ]
        },
        { text: 'Run a process engine', link: 'run-process-engine' },
        { text: 'Using CLI', link: 'using-cli' },
      ]
    },
    {
      text: 'Reference',
      collapsed: false,
      items: [
        { text: 'BPMN 2.0', link: 'bpmn20' },
        { text: 'API documentation', link: 'api-documentation' },
      ]
    },
  ]
}

function preprocessCode() {
  const code = {}
  for (const artifact of release.artifacts) {
    if (artifact.os == 'windows') {
      code[`download-${artifact.osarch}`] = `curl.exe -L -o go-bpmn-${artifact.osarch}.tar.gz "${githubUrl}/releases/download/${release.version}/go-bpmn-${artifact.osarch}.tar.gz"`
      code[`validate-${artifact.osarch}`] = `(Get-FileHash go-bpmn-${artifact.osarch}.tar.gz).Hash -eq "${artifact.checksum}"`
      code[`extract-${artifact.osarch}`]  = `tar.exe -xvzf go-bpmn-${artifact.osarch}.tar.gz`
    } else {
      code[`download-${artifact.osarch}`] = `curl -L -o go-bpmn-${artifact.osarch}.tar.gz ${githubUrl}/releases/download/${release.version}/go-bpmn-${artifact.osarch}.tar.gz`
      code[`validate-${artifact.osarch}`] = `echo "${artifact.checksum} go-bpmn-${artifact.osarch}.tar.gz" | sha256sum -c`
      code[`extract-${artifact.osarch}`]  = `tar -xvzf go-bpmn-${artifact.osarch}.tar.gz`
    }
  }
  
  code['download-openapi-linux']   = `curl -L -o go-bpmn-openapi.yaml ${githubUrl}/releases/download/${release.version}/go-bpmn-openapi.yaml`
  code['download-openapi-windows'] = `curl.exe -L -o go-bpmn-openapi.yaml "${githubUrl}/releases/download/${release.version}/go-bpmn-openapi.yaml"`

  return code
}
