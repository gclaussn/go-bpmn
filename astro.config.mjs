// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
  site: 'https://gclaussn.github.io',
  base: 'go-bpmn',
  integrations: [
    starlight({
      title: 'go-bpmn',
      social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/gclaussn/go-bpmn' }],
      sidebar: [
        {
          label: 'Getting started',
          items: [
            { label: 'Introduction', slug: 'getting-started/introduction' },
            { label: 'Automate a process', slug: 'getting-started/automate-process' },
          ],
        },
        {
          label: 'Guides',
          items: [
            { label: 'Installation', slug: 'guides/installation' },
            { label: 'Run a process engine', slug: 'guides/run-process-engine' },
            { label: 'Using CLI', slug: 'guides/using-cli' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'BPMN 2.0 coverage', slug: 'reference/bpmn-coverage' },
            { label: 'API documentation', slug: 'reference/api-documentation' },
          ],
        },
      ],
    }),
  ],
});
