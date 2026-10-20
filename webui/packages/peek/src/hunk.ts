/** The chat message for a hunk comment: the file and hunk header, the hunk as a diff block, then the comment. */
export function hunkCommentMessage(comment: { path: string; header: string; hunkText: string; comment: string }): string {
  return `\`${comment.path}\` \`${comment.header}\`\n\n\`\`\`diff\n${comment.hunkText}\n\`\`\`\n\n${comment.comment}`
}
