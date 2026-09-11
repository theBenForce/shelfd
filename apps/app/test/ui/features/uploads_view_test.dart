import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:shelf/data/models/upload_job.dart';
import 'package:shelf/ui/core/theme.dart';
import 'package:shelf/ui/features/upload/uploads_view.dart';
import 'package:shelf/ui/state/providers.dart';

class FakeUploadNotifier extends UploadNotifier {
  final UploadState initialState;
  FakeUploadNotifier(this.initialState);

  @override
  UploadState build() => initialState;

  @override
  Future<void> loadStagedJobs() async {}
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late SharedPreferences prefs;

  setUp(() async {
    SharedPreferences.setMockInitialValues({});
    prefs = await SharedPreferences.getInstance();
  });

  testWidgets('UploadsView renders header, dropzone, and empty state', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });

    const state = UploadState(
      stagedJobs: [],
      autoCommit: false,
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          uploadProvider.overrideWith(() => FakeUploadNotifier(state)),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const UploadsView(),
        ),
      ),
    );

    expect(find.text('Uploads & Ingestion'), findsOneWidget);
    expect(find.text('Auto-add'), findsOneWidget);
    expect(find.text('Drag & drop EPUB files or nested folders here'), findsOneWidget);
    expect(find.text('Browse EPUB Files'), findsOneWidget);
    expect(find.text('Select Folder'), findsOneWidget);
    expect(find.text('No Pending Uploads'), findsOneWidget);
  });

  testWidgets('UploadsView displays batch staging progress when isUploading is true', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });

    const state = UploadState(
      isUploading: true,
      uploadProgress: 0.6,
      currentBatchStatus: 'Staging 3 of 5: Earthsea.epub',
      stagedJobs: [],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          uploadProvider.overrideWith(() => FakeUploadNotifier(state)),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const UploadsView(),
        ),
      ),
    );

    expect(find.text('Staging 3 of 5: Earthsea.epub'), findsOneWidget);
    expect(find.text('60%'), findsOneWidget);
    expect(find.byType(LinearProgressIndicator), findsOneWidget);
  });

  testWidgets('UploadsView renders staged queue and metadata inspector on desktop', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });

    const job1 = StagedUploadJob(
      jobId: 'job-1',
      status: 'staged',
      filename: 'dune.epub',
      metadata: StagedMetadata(
        title: 'Dune',
        authors: ['Frank Herbert'],
        series: 'Dune Chronicles',
        sequenceNumber: 1,
      ),
      hasCover: false,
      warnings: [],
    );

    const job2 = StagedUploadJob(
      jobId: 'job-2',
      status: 'staged',
      filename: 'unknown.epub',
      metadata: StagedMetadata(
        title: 'Untitled',
        authors: [],
      ),
      hasCover: false,
      warnings: ['No author found in EPUB metadata'],
    );

    const state = UploadState(
      stagedJobs: [job1, job2],
      selectedJobId: 'job-1',
      autoCommit: false,
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          sharedPreferencesProvider.overrideWithValue(prefs),
          uploadProvider.overrideWith(() => FakeUploadNotifier(state)),
        ],
        child: MaterialApp(
          theme: AppTheme.buildTheme(ReadingThemeMode.bone),
          home: const UploadsView(),
        ),
      ),
    );

    // Queue counts and pills
    expect(find.text('Queue (2)'), findsOneWidget);
    expect(find.text('1 Ready'), findsOneWidget);
    expect(find.text('1 Incomplete'), findsOneWidget);
    expect(find.text('Approve All Ready (1)'), findsOneWidget);

    // Job cards
    expect(find.text('Dune'), findsAtLeastNWidgets(1));
    expect(find.text('Frank Herbert'), findsAtLeastNWidgets(1));
    expect(find.text('dune.epub'), findsOneWidget);
    expect(find.text('No author found in EPUB metadata'), findsOneWidget);

    // Metadata Inspector
    expect(find.text('Metadata Inspector'), findsOneWidget);
    expect(find.text('Target Library Destination'), findsOneWidget);
    expect(find.text('/library/Frank Herbert/Dune/Dune.epub'), findsOneWidget);
    expect(find.text('Save & Add to Library'), findsOneWidget);
  });
}
