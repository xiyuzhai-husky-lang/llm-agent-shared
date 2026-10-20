import type { KyDocumentWire } from '@proposal/kathy-document'
export interface Candidate {proposal_number:number;candidate_commit:string}
/** One hunk of one file; `hunk` is empty for a file without hunks. */
export interface Position {path:string;hunk:string}
export interface Finding {index:number;identifier:string;anchors:{path:string;range:string|null}[];ruling:string|null;display:{document:KyDocumentWire;references:{label:string;path:string;line:number;end_line:number|null}[]}}
export type Signature = {signer:string;at:string} & ({carried?:never;signedAtCandidate?:never}|{carried:true;signedAtCandidate:string})
export interface SignatureState {path:string;hunk:string;range:string;ordinal:number;signature?:Signature;changedLines:number}
export interface Line {old:number|null;new:number|null;kind:string;spans:{text:string;token_class:string|null}[]}
export interface Stats {files_label:string|null;hunks_label:string;hunks_title:string;hunks_aria_label:string|null;additions_label:string|null;deletions_label:string|null;aria_label:string}
export interface GroupHeading {label:string;aria_label:string;empty:boolean;first_position:Position|null;stats:Stats}
/** A change kind as marked: `class` selects the style, `label` is the word shown. */
export interface Change {class:string;label:string}
export interface TreeNode {kind:'directory'|'file';key:string;group:string;path:string;label:string;depth:number;title:string;aria_label:string;change:Change|null;stats:Stats;opens:Position|null}
/** The rows of one directory, answered when the directory is opened. */
export interface ChildrenView {generation:number;nodes:TreeNode[]}
export interface KindGroup {kind:string;heading:GroupHeading;classes:{key:string;heading:GroupHeading;subclasses:{key:string;heading:GroupHeading;nodes:TreeNode[]}[]}[]}
export interface FileLabels {display_path:string;change:Change;focused_label:string|null}
export interface BatchSignMenu {operations:{directory:string;label:string;unsigned_count:number}[]}
/** One file of the Diff, highlighted; the server produces it when the file is opened. */
export interface FileDocument {path:string;notice:string|null;body_notice:string|null;blocks:{hunk:string|null;lines:Line[]}[]}
/** One hunk of an opened file with its review state and its neighbours in the order the tab walks. */
export interface HunkView {
 header:string;header_line:string;label:string;must_sign_hunk_ordinal:number|null;
 signature:SignatureState|null;
 /** The review panel's sections; each heading already counts the file-wide and proposal-wide items listed below. */
 review_sections:{label:string;heading:string;empty_notice:string|null;findings:Finding[]}[];
 file_wide_findings:Finding[];proposal_wide_findings:Finding[];
 up:Position|null;down:Position|null;next_action:Position|null;
 /** Where the page goes once this hunk is signed, and once a finding is ruled at it. */
 after_sign:Position|null;after_ruling:Position|null
}
/** One file of the Diff, answered when the file is opened. */
export interface FileView {
 generation:number;path:string;labels:FileLabels;
 /** The one-sentence reason when the file was added to MUST SIGN beyond the patterns, with the label saying who added it. */
 must_sign_reason:{label:string;reason:string}|null;
 tree_group:string;tree_directories:string[];hunks:HunkView[];
 /** What the citation the page asked with opens: the hunk to focus and the candidate-side lines to mark. */
 cited:{line:number;hunk:string|null;lines:number[]}|null;
 file_wide_findings:Finding[];next_action:Position|null;batch_sign_menu:BatchSignMenu|null
}
/** The Diff tab as the page connection carries it: the top of the tree and what is not per file. */
export interface DiffTabView {
 candidate:Candidate;generation:number;tree:KindGroup[];first_position:Position|null;
 sign_presses:number;write:SignatureWrite|null;finding_rulings:{identifier:string;ruling:string|null}[];
 proposal_wide_findings:Finding[];review_comment_count:number
}

export type SignatureWrite = {path:string;hunk:string;changedLines:number;requiredWaitMilliseconds:number} & (
 {outcome:'signed'|'already_signed';remainingMs?:never;message?:never}
 | {outcome:'refused_as_too_fast';remainingMs:number;message:string}
)
