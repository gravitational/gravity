function handleVerSelector() {
  if (!window.grvConfig || !window.grvConfig.docVersions) {
    return;
  }

  var docVersions = window.grvConfig.docVersions || [];
  var docCurrentVer = window.grvConfig.docCurrentVer;

  function getVerUrl(ver, isLatest) {
    // looks for version number and replaces it with new value
    // ex: http://host/docs/ver/1.2/review -> http://host/docs/ver/4.0
    //var reg = new RegExp("\/ver\/([0-9|\.]+(?=\/.))");
    var reg = new RegExp("\/ver\/(.*)\/");
    var url = window.location.href.replace(reg, '');
    var newPrefix = isLatest ? "" : "/ver/" + ver +"/";
    return url.replace(mkdocs_page_url, newPrefix);
  }

  var $options = [];
  // show links to other versions
  for (var i = 0; i < docVersions.length; i++) {
    var ver = docVersions[i];
    var $li = null;
    var isCurrent = docCurrentVer === ver;
    if (isCurrent) {
      curValue = ver;
      $options.push('<option selected value="' + ver + '" >v' + ver + "</option>"  );
      continue;
    }

    var isLatest = docVersions.indexOf(ver) === (docVersions.length - 1);
    var baseUrl = getVerUrl(ver, isLatest);
    $options.push(' <option value="' + baseUrl + '" >v' + ver + "</option>");
  }

  var $container = $(".rst-content");
  var $versionList = $(
    '<form name="grv-ver-selector" class="grv-ver-selector">' +
      '<label for="menu">Version</label>' +
      '<select name="menu" onChange="window.document.location.href=this.options[this.selectedIndex].value;" value="' + curValue + '">'
        + $options.reverse().join('') +
      '</select>' +
    '</form>'
  );

  // show warning if older version
  var isLatest =
    docVersions.length === 0 ||
    docCurrentVer === docVersions[docVersions.length - 1];

  if (!isLatest) {
    var latestVerUrl = getVerUrl(docVersions[docVersions.length - 1], true);
    $('div [role="main"] div.section h1').after(
      '<div class="admonition warning" style="margin: 0px 0 15px 0;"> ' +
      '   <p class="admonition-title">Version Warning</p> ' +
      '   <p>This chapter covers Gravity ' + docCurrentVer +'. We highly recommend evaluating ' +
      '   the <a href="' + latestVerUrl + '">latest</a> version instead.</p> ' +
      '</div>'
    );
  }

  $container.prepend($versionList);
}


function init(fn, description) {
  try {
    fn()
  } catch (err) {
    console.error('failed to init ' + description, err);
  }
}

$( document ).ready(function() {
    init(handleVerSelector, "handleVerSelector");

    // Shift nav in mobile when clicking the menu.
    $(document).on('click', "[data-toggle='wy-nav-top']", function() {
      $("[data-toggle='wy-nav-shift']").toggleClass("shift");
      $("[data-toggle='rst-versions']").toggleClass("shift");
    });

    // Close menu when you click a link.
    $(document).on('click', ".wy-menu-vertical .current ul li a", function() {
      $("[data-toggle='wy-nav-shift']").removeClass("shift");
      $("[data-toggle='rst-versions']").toggleClass("shift");
    });

    $(document).on('click', "[data-toggle='rst-current-version']", function() {
      $("[data-toggle='rst-versions']").toggleClass("shift-up");
    });

    // Make tables responsive
    $("table.docutils:not(.field-list)").wrap("<div class='wy-table-responsive'></div>");

    hljs.initHighlightingOnLoad();

    $('table').addClass('docutils');
});

