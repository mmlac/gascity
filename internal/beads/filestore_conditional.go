package beads

// FileStore embeds *MemStore, which implements ConditionalWriter — so without
// these shadows the promoted MemStore CAS methods would satisfy the interface
// on a FileStore while writing straight to the in-memory MemStore, bypassing
// FileStore's flush-on-write, cross-process flock, and reload-before-write. A
// conditional write made that way is silently lost on the next reload.
//
// Until the native FileStore implementation lands (S2-T3-file: reload → snapshot
// → delegate → save → rollback, plus out-of-band revision persistence because
// Bead.Revision is json:"-" and never survives the on-disk []Bead), FileStore
// reports itself incapable of conditional writes rather than exposing the broken
// promoted writer. The interface stays satisfied so discovery via
// ConditionalWriterFor is honest; the methods refuse.
var _ ConditionalWriter = (*FileStore)(nil)

// UpdateIfMatch reports conditional writes unsupported until the native FileStore
// implementation lands (S2-T3-file).
func (fs *FileStore) UpdateIfMatch(_ string, _ int64, _ UpdateOpts) error {
	return ErrConditionalWriteUnsupported
}

// CloseIfMatch reports conditional writes unsupported until the native FileStore
// implementation lands (S2-T3-file).
func (fs *FileStore) CloseIfMatch(_ string, _ int64) error {
	return ErrConditionalWriteUnsupported
}

// DeleteIfMatch reports conditional writes unsupported until the native FileStore
// implementation lands (S2-T3-file).
func (fs *FileStore) DeleteIfMatch(_ string, _ int64) error {
	return ErrConditionalWriteUnsupported
}

// CompareAndSetMetadataKey reports conditional writes unsupported until the
// native FileStore implementation lands (S2-T3-file).
func (fs *FileStore) CompareAndSetMetadataKey(_, _, _, _ string) (bool, error) {
	return false, ErrConditionalWriteUnsupported
}
