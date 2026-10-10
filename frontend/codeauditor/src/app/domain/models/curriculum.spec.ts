// curriculum.spec.ts — domain type contract tests for the free-practice
// feature. The types under test live in domain/models/curriculum.ts and
// must remain framework-free (zero @angular/* imports per the project's
// layer rule). The shape mirrors the wire of `mcp-curriculum.get_topics`
// (see openspec/changes/free-practice-mode/design.md §Data Shapes).
//
// The RED state of this file: importing `CurriculumTopic` and
// `GenerateChallengeArgs` from a non-existent module — the spec runner
// fails to load the test before any assertion fires. GREEN: the
// module exists, every fixture compiles, and the structural assertions
// pass.
import { describe, it, expect } from 'vitest';

import { CurriculumTopic, GenerateChallengeArgs, F_S_LEVELS, isF_SLevel } from './curriculum';

describe('CurriculumTopic', () => {
  it('accepts a canonical typescript topic fixture', () => {
    const topic: CurriculumTopic = {
      id: 'inheritance',
      title: 'Inheritance',
      difficulty: 'D',
      description: 'Subclassing, super calls, and overriding in TS.',
    };
    expect(topic.id).toBe('inheritance');
    expect(topic.difficulty).toBe('D');
  });

  it('keeps every F..S level assignable to difficulty', () => {
    for (const lvl of F_S_LEVELS) {
      const topic: CurriculumTopic = {
        id: `topic-${lvl}`,
        title: `Topic ${lvl}`,
        difficulty: lvl,
        description: 'fixture',
      };
      expect(topic.difficulty).toBe(lvl);
    }
  });
});

describe('GenerateChallengeArgs', () => {
  it('mirrors the mcp-challenges.generate_challenge wire shape', () => {
    const args: GenerateChallengeArgs = {
      language: 'typescript',
      topicId: 'inheritance',
      level: 'D',
    };
    expect(args.language).toBe('typescript');
    expect(args.topicId).toBe('inheritance');
    expect(args.level).toBe('D');
  });
});

describe('isF_SLevel', () => {
  it('accepts every letter in F..S', () => {
    expect(F_S_LEVELS).toEqual(['F', 'E', 'D', 'C', 'B', 'A', 'S']);
    for (const letter of F_S_LEVELS) {
      expect(isF_SLevel(letter)).toBe(true);
    }
  });

  it('rejects letters outside the F..S band', () => {
    for (const letter of ['Z', 'M', 'X', '', 'f', 'd', ' ', 'AA']) {
      expect(isF_SLevel(letter)).toBe(false);
    }
  });
});
