#define AppName "VidDock"
#define AppVersion "0.2.2"
#define Publisher "VidDock"
#define AppExeName "VidDockHelper.exe"

[Setup]
AppId={{D40AB2A6-7115-4E18-9487-7FE7A122A9D4}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#Publisher}
DefaultDirName={localappdata}\Programs\VidDock
DefaultGroupName=VidDock
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\dist
OutputBaseFilename=VidDock-Setup-{#AppVersion}
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
SetupIconFile=..\assets\VidDock.ico
UninstallDisplayIcon={app}\VidDock.ico
VersionInfoVersion={#AppVersion}.0
VersionInfoCompany={#Publisher}
VersionInfoDescription=VidDock Installer
VersionInfoProductName={#AppName}
ChangesAssociations=no
CloseApplications=no
SetupMutex=VidDock.Setup.D40AB2A6-7115-4E18-9487-7FE7A122A9D4

[Files]
Source: "..\dist\helper\VidDockHelper.exe"; DestDir: "{app}"; Flags: ignoreversion restartreplace uninsrestartdelete
Source: "..\dist\helper\VidDockHelper.exe"; DestName: "VidDockLifecycle.exe"; Flags: dontcopy
Source: "..\dist\helper\tools\yt-dlp.exe"; DestDir: "{app}\tools"; Flags: ignoreversion restartreplace uninsrestartdelete
Source: "..\dist\helper\tools\ffmpeg.exe"; DestDir: "{app}\tools"; Flags: ignoreversion restartreplace uninsrestartdelete
Source: "..\dist\helper\tools\ffprobe.exe"; DestDir: "{app}\tools"; Flags: ignoreversion restartreplace uninsrestartdelete
Source: "..\extension\*"; DestDir: "{app}\Extension"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "package-version.json"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\assets\VidDock.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\SECURITY.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\THIRD_PARTY_NOTICES.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\licenses\*.txt"; DestDir: "{app}\licenses"; Flags: ignoreversion

[Dirs]
Name: "{localappdata}\VidDock\logs"

[UninstallDelete]
Type: files; Name: "{app}\native-host\com.viddock.helper.json"
Type: dirifempty; Name: "{app}\native-host"

[Icons]
Name: "{group}\VidDock Extension Folder"; Filename: "{app}\Extension"
Name: "{group}\Repair Browser Integration"; Filename: "{app}\VidDockHelper.exe"; Parameters: "--repair-ui"; WorkingDir: "{app}"
Name: "{group}\VidDock README"; Filename: "{app}\README.md"
Name: "{group}\Uninstall VidDock"; Filename: "{uninstallexe}"

[Registry]
Root: HKCU; Subkey: "Software\Google\Chrome\NativeMessagingHosts\com.viddock.helper"; ValueType: string; ValueName: ""; ValueData: "{app}\native-host\com.viddock.helper.json"; Flags: uninsdeletekey; Check: ConfigureChrome
Root: HKCU; Subkey: "Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\com.viddock.helper"; ValueType: string; ValueName: ""; ValueData: "{app}\native-host\com.viddock.helper.json"; Flags: uninsdeletekey; Check: ConfigureBrave
Root: HKCU; Subkey: "Software\Microsoft\Edge\NativeMessagingHosts\com.viddock.helper"; ValueType: string; ValueName: ""; ValueData: "{app}\native-host\com.viddock.helper.json"; Flags: uninsdeletekey; Check: ConfigureEdge

[Run]
Filename: "{app}\VidDockHelper.exe"; Parameters: "--open-browser-setup"; Description: "Open browser extension setup"; Flags: postinstall nowait skipifsilent unchecked

[Code]
var
  BrowserPage: TInputOptionWizardPage;
  ChromeInstalled, BraveInstalled, EdgeInstalled: Boolean;
  LifecycleQuiesced, InstallCompleted: Boolean;
  OldChromeExists, OldBraveExists, OldEdgeExists: Boolean;
  OldChromeValue, OldBraveValue, OldEdgeValue: String;

const
  ChromeNativeKey = 'Software\Google\Chrome\NativeMessagingHosts\com.viddock.helper';
  BraveNativeKey = 'Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\com.viddock.helper';
  EdgeNativeKey = 'Software\Microsoft\Edge\NativeMessagingHosts\com.viddock.helper';

procedure SaveAndRemoveIntegration;
begin
  OldChromeExists := RegQueryStringValue(HKEY_CURRENT_USER, ChromeNativeKey, '', OldChromeValue);
  OldBraveExists := RegQueryStringValue(HKEY_CURRENT_USER, BraveNativeKey, '', OldBraveValue);
  OldEdgeExists := RegQueryStringValue(HKEY_CURRENT_USER, EdgeNativeKey, '', OldEdgeValue);
  RegDeleteKeyIncludingSubkeys(HKEY_CURRENT_USER, ChromeNativeKey);
  RegDeleteKeyIncludingSubkeys(HKEY_CURRENT_USER, BraveNativeKey);
  RegDeleteKeyIncludingSubkeys(HKEY_CURRENT_USER, EdgeNativeKey);
end;

procedure SaveIntegration;
begin
  OldChromeExists := RegQueryStringValue(HKEY_CURRENT_USER, ChromeNativeKey, '', OldChromeValue);
  OldBraveExists := RegQueryStringValue(HKEY_CURRENT_USER, BraveNativeKey, '', OldBraveValue);
  OldEdgeExists := RegQueryStringValue(HKEY_CURRENT_USER, EdgeNativeKey, '', OldEdgeValue);
end;

procedure RestoreSavedIntegration;
begin
  if OldChromeExists then RegWriteStringValue(HKEY_CURRENT_USER, ChromeNativeKey, '', OldChromeValue);
  if OldBraveExists then RegWriteStringValue(HKEY_CURRENT_USER, BraveNativeKey, '', OldBraveValue);
  if OldEdgeExists then RegWriteStringValue(HKEY_CURRENT_USER, EdgeNativeKey, '', OldEdgeValue);
end;

function RunLifecycle(Argument, Target: String; var ResultCode: Integer): Boolean;
var
  Parameters: String;
begin
  Parameters := Argument;
  if Target <> '' then Parameters := Parameters + ' "' + Target + '"';
  Result := Exec(ExpandConstant('{tmp}\VidDockLifecycle.exe'), Parameters, '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  Target, PromptText: String;
  StatusCode, StopCode: Integer;
begin
  Result := '';
  Target := ExpandConstant('{app}\VidDockHelper.exe');
  if not FileExists(Target) then Exit;
  ExtractTemporaryFile('VidDockLifecycle.exe');
  StatusCode := 0;
  if not RunLifecycle('--lifecycle-status', Target, StatusCode) then
  begin
    Result := 'VidDock could not inspect the existing helper process.';
    Exit;
  end;
  Log(Format('Existing helper lifecycle status: %d', [StatusCode]));
  if (StatusCode <> 0) and (StatusCode <> 10) and (StatusCode <> 11) and (StatusCode <> 12) then
  begin
    Result := 'VidDock could not validate the existing helper process.';
    Exit;
  end;
  if (StatusCode = 11) or (StatusCode = 12) then
  begin
    if StatusCode = 11 then
      PromptText := 'VidDock is currently downloading or merging a file. Continuing will safely cancel VidDock operations and remove their private temporary fragments before upgrading. Finished downloads will not be removed.'
    else
      PromptText := 'An older VidDock helper is running. Continuing will close it before upgrading. Any operation using that helper will be cancelled safely.';
    if (not WizardSilent) and (MsgBox(PromptText + #13#10 + #13#10 + 'Continue with the upgrade?', mbConfirmation, MB_YESNO) <> IDYES) then
    begin
      Result := 'Close or finish the current VidDock operation, then run Setup again.';
      Exit;
    end;
  end;
  SaveIntegration;
  LifecycleQuiesced := True;
  StopCode := 0;
  if (not RunLifecycle('--lifecycle-stop', Target, StopCode)) or (StopCode <> 0) then
  begin
    RunLifecycle('--lifecycle-resume', '', StatusCode);
    RestoreSavedIntegration;
    LifecycleQuiesced := False;
    Result := 'VidDockHelper.exe could not be closed safely. Setup has restored browser integration and made no file changes.';
    Exit;
  end;
  Log('VidDock helper processes stopped; existing Native Messaging registration was preserved.');
end;

function IsChromeInstalled: Boolean;
begin
  Result := FileExists(ExpandConstant('{pf64}\Google\Chrome\Application\chrome.exe')) or
    FileExists(ExpandConstant('{pf32}\Google\Chrome\Application\chrome.exe')) or
    FileExists(ExpandConstant('{localappdata}\Google\Chrome\Application\chrome.exe'));
end;

function IsBraveInstalled: Boolean;
begin
  Result := FileExists(ExpandConstant('{pf64}\BraveSoftware\Brave-Browser\Application\brave.exe')) or
    FileExists(ExpandConstant('{pf32}\BraveSoftware\Brave-Browser\Application\brave.exe')) or
    FileExists(ExpandConstant('{localappdata}\BraveSoftware\Brave-Browser\Application\brave.exe'));
end;

function IsEdgeInstalled: Boolean;
begin
  Result := FileExists(ExpandConstant('{pf64}\Microsoft\Edge\Application\msedge.exe')) or
    FileExists(ExpandConstant('{pf32}\Microsoft\Edge\Application\msedge.exe')) or
    FileExists(ExpandConstant('{localappdata}\Microsoft\Edge\Application\msedge.exe'));
end;

function DetectionLabel(Name: String; Installed: Boolean): String;
begin
  if Installed then
    Result := Name + ' - detected'
  else
    Result := Name + ' - not detected';
end;

procedure InitializeWizard;
begin
  ChromeInstalled := IsChromeInstalled;
  BraveInstalled := IsBraveInstalled;
  EdgeInstalled := IsEdgeInstalled;
  BrowserPage := CreateInputOptionPage(wpSelectDir, 'Browser Integration',
    'Choose detected browsers',
    'VidDock will register its restricted Native Messaging host for selected installed browsers. Browser security still requires you to approve the local extension.', False, False);
  BrowserPage.Add(DetectionLabel('Google Chrome', ChromeInstalled));
  BrowserPage.Add(DetectionLabel('Brave Browser', BraveInstalled));
  BrowserPage.Add(DetectionLabel('Microsoft Edge', EdgeInstalled));
  BrowserPage.Values[0] := ChromeInstalled;
  BrowserPage.Values[1] := BraveInstalled;
  BrowserPage.Values[2] := EdgeInstalled;
end;

function ConfigureChrome: Boolean;
begin
  Result := ChromeInstalled and BrowserPage.Values[0];
end;

function ConfigureBrave: Boolean;
begin
  Result := BraveInstalled and BrowserPage.Values[1];
end;

function ConfigureEdge: Boolean;
begin
  Result := EdgeInstalled and BrowserPage.Values[2];
end;

function IntegrationStatus(Name: String; Installed, Selected: Boolean): String;
begin
  if not Installed then
    Result := Name + ': Not detected'
  else if Selected then
    Result := Name + ': Native helper ready; Load unpacked approval required'
  else
    Result := Name + ': Detected; integration not selected';
end;

procedure CurPageChanged(CurPageID: Integer);
begin
  if CurPageID = wpFinished then
    WizardForm.FinishedLabel.Caption :=
      'VidDock installed successfully.' + #13#10 + #13#10 +
      IntegrationStatus('Chrome', ChromeInstalled, ConfigureChrome) + #13#10 +
      IntegrationStatus('Brave', BraveInstalled, ConfigureBrave) + #13#10 +
      IntegrationStatus('Edge', EdgeInstalled, ConfigureEdge) + #13#10 + #13#10 +
      'Native Helper: Installed' + #13#10 +
      'yt-dlp: Installed' + #13#10 +
      'FFmpeg: Installed' + #13#10 +
      'Extension files: Updated in place' + #13#10 + #13#10 +
      'VidDock requests a supported extension reload when an active older build detects this installed package. If the browser keeps stale unpacked code, completely exit and reopen it; click Reload only as a final fallback.' + #13#10 + #13#10 +
      'Use the optional browser setup action below only when initial extension approval or manual fallback is required.';
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  ResultCode: Integer;
begin
  if CurStep = ssPostInstall then
  begin
    if FileExists(ExpandConstant('{app}\VidDockHelper.exe')) then
    begin
      if (not Exec(ExpandConstant('{app}\VidDockHelper.exe'), '--repair', '', SW_HIDE, ewWaitUntilTerminated, ResultCode)) or (ResultCode <> 0) then
        RaiseException('VidDock post-install Native Messaging validation failed. Run Repair Browser Integration or reinstall.');
    end;
    LifecycleQuiesced := False;
    InstallCompleted := True;
  end;
end;

procedure DeinitializeSetup;
var
  ResultCode: Integer;
begin
  if LifecycleQuiesced and (not InstallCompleted) then
  begin
    if FileExists(ExpandConstant('{tmp}\VidDockLifecycle.exe')) then
      RunLifecycle('--lifecycle-resume', '', ResultCode);
    RestoreSavedIntegration;
  end;
end;

function InitializeUninstall: Boolean;
var
  Target, PromptText: String;
  StatusCode, StopCode: Integer;
begin
  Result := False;
  Target := ExpandConstant('{app}\VidDockHelper.exe');
  if not FileExists(Target) then begin Result := True; Exit; end;
  StatusCode := 0;
  if not Exec(Target, '--lifecycle-status "' + Target + '"', '', SW_HIDE, ewWaitUntilTerminated, StatusCode) then Exit;
  if (StatusCode <> 0) and (StatusCode <> 10) and (StatusCode <> 11) and (StatusCode <> 12) then
  begin
    MsgBox('VidDock could not verify its running processes. Uninstall was cancelled without changing browser integration.', mbError, MB_OK);
    Exit;
  end;
  if (StatusCode = 11) or (StatusCode = 12) then
  begin
    PromptText := 'VidDock is currently active. Uninstalling will safely cancel its operations and remove private temporary fragments. Finished downloads will not be removed.';
    if (not UninstallSilent) and (MsgBox(PromptText + #13#10 + #13#10 + 'Continue uninstalling?', mbConfirmation, MB_YESNO) <> IDYES) then Exit;
  end;
  SaveAndRemoveIntegration;
  StopCode := 0;
  if (not Exec(Target, '--lifecycle-stop "' + Target + '"', '', SW_HIDE, ewWaitUntilTerminated, StopCode)) or (StopCode <> 0) then
  begin
    Exec(Target, '--lifecycle-resume', '', SW_HIDE, ewWaitUntilTerminated, StatusCode);
    RestoreSavedIntegration;
    MsgBox('VidDockHelper.exe could not be closed safely. Uninstall was cancelled and browser integration was restored.', mbError, MB_OK);
    Exit;
  end;
  Result := True;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usUninstall then
  begin
    RegDeleteKeyIncludingSubkeys(HKEY_CURRENT_USER, ChromeNativeKey);
    RegDeleteKeyIncludingSubkeys(HKEY_CURRENT_USER, BraveNativeKey);
    RegDeleteKeyIncludingSubkeys(HKEY_CURRENT_USER, EdgeNativeKey);
  end;
end;
